package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS metric_samples (
	recorded_at INTEGER PRIMARY KEY,
	server_fps INTEGER NOT NULL,
	current_players INTEGER NOT NULL,
	server_frame_time REAL NOT NULL,
	max_players INTEGER NOT NULL,
	uptime_seconds INTEGER NOT NULL,
	base_camps INTEGER NOT NULL,
	world_days INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS metric_samples_recorded_at_idx
	ON metric_samples(recorded_at DESC);

CREATE TABLE IF NOT EXISTS player_stat_samples (
	player_key TEXT NOT NULL,
	recorded_at_ms INTEGER NOT NULL,
	name TEXT NOT NULL,
	account_name TEXT NOT NULL,
	player_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	online INTEGER NOT NULL CHECK (online IN (0, 1)),
	location_x REAL,
	location_y REAL,
	level INTEGER,
	PRIMARY KEY (player_key, recorded_at_ms)
);
CREATE INDEX IF NOT EXISTS player_stat_samples_recorded_at_idx
	ON player_stat_samples(recorded_at_ms DESC);
CREATE INDEX IF NOT EXISTS player_stat_samples_user_id_idx
	ON player_stat_samples(user_id, recorded_at_ms DESC);

CREATE TABLE IF NOT EXISTS player_stat_state (
	player_key TEXT PRIMARY KEY,
	recorded_at_ms INTEGER NOT NULL,
	name TEXT NOT NULL,
	account_name TEXT NOT NULL,
	player_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	online INTEGER NOT NULL CHECK (online IN (0, 1))
);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	return nil
}

func (s *Store) RecordMetrics(ctx context.Context, metrics domain.Metrics) error {
	_, err := s.db.ExecContext(ctx, `
INSERT OR REPLACE INTO metric_samples (
	recorded_at, server_fps, current_players, server_frame_time,
	max_players, uptime_seconds, base_camps, world_days
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		metrics.RecordedAt.Unix(),
		metrics.ServerFPS,
		metrics.CurrentPlayerNum,
		metrics.ServerFrameTime,
		metrics.MaxPlayerNum,
		metrics.Uptime,
		metrics.BaseCampNum,
		metrics.Days,
	)
	if err != nil {
		return fmt.Errorf("record metrics: %w", err)
	}
	return nil
}

// RecordPlayerStats records the current online players and an offline transition
// for anyone who was online in the previous successful snapshot.
func (s *Store) RecordPlayerStats(ctx context.Context, players []domain.Player, recordedAt time.Time) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin player stat transaction: %w", err)
	}
	defer tx.Rollback()

	previouslyOnline, err := onlinePlayerStates(ctx, tx)
	if err != nil {
		return err
	}

	recordedAtMS := recordedAt.UnixMilli()
	seen := make(map[string]struct{}, len(players))
	for _, player := range players {
		key := playerKey(player)
		if key == "" {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		delete(previouslyOnline, key)

		if _, err = tx.ExecContext(ctx, `
INSERT OR REPLACE INTO player_stat_samples (
	player_key, recorded_at_ms, name, account_name, player_id, user_id, online,
	location_x, location_y, level
) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
			key,
			recordedAtMS,
			player.Name,
			player.AccountName,
			player.PlayerID,
			player.UserID,
			player.LocationX,
			player.LocationY,
			player.Level,
		); err != nil {
			return fmt.Errorf("record online player stat: %w", err)
		}
		if err = upsertPlayerState(ctx, tx, key, recordedAtMS, player.Name, player.AccountName, player.PlayerID, player.UserID, true); err != nil {
			return err
		}
	}

	for key, player := range previouslyOnline {
		if _, err = tx.ExecContext(ctx, `
INSERT OR REPLACE INTO player_stat_samples (
	player_key, recorded_at_ms, name, account_name, player_id, user_id, online,
	location_x, location_y, level
) VALUES (?, ?, ?, ?, ?, ?, 0, NULL, NULL, NULL)`,
			key,
			recordedAtMS,
			player.name,
			player.accountName,
			player.playerID,
			player.userID,
		); err != nil {
			return fmt.Errorf("record offline player stat: %w", err)
		}
		if err = upsertPlayerState(ctx, tx, key, recordedAtMS, player.name, player.accountName, player.playerID, player.userID, false); err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit player stats: %w", err)
	}
	return nil
}

func (s *Store) RecentPlayerStats(ctx context.Context, since time.Time, limit int) ([]domain.PlayerStat, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT player_key, recorded_at_ms, name, account_name, player_id, user_id, online,
	location_x, location_y, level
FROM player_stat_samples
WHERE recorded_at_ms >= ?
ORDER BY recorded_at_ms DESC, player_key
LIMIT ?`, since.UnixMilli(), limit)
	if err != nil {
		return nil, fmt.Errorf("query recent player stats: %w", err)
	}
	defer rows.Close()

	var samples []domain.PlayerStat
	for rows.Next() {
		var (
			recordedAtMS int64
			online       bool
			locationX    sql.NullFloat64
			locationY    sql.NullFloat64
			level        sql.NullInt64
			sample       domain.PlayerStat
		)
		if err := rows.Scan(
			&sample.PlayerKey,
			&recordedAtMS,
			&sample.Name,
			&sample.AccountName,
			&sample.PlayerID,
			&sample.UserID,
			&online,
			&locationX,
			&locationY,
			&level,
		); err != nil {
			return nil, fmt.Errorf("scan recent player stat: %w", err)
		}
		sample.Online = online
		sample.RecordedAt = time.UnixMilli(recordedAtMS)
		if locationX.Valid {
			sample.LocationX = &locationX.Float64
		}
		if locationY.Valid {
			sample.LocationY = &locationY.Float64
		}
		if level.Valid {
			value := int(level.Int64)
			sample.Level = &value
		}
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent player stats: %w", err)
	}
	return samples, nil
}

type storedPlayerState struct {
	name        string
	accountName string
	playerID    string
	userID      string
}

func onlinePlayerStates(ctx context.Context, tx *sql.Tx) (map[string]storedPlayerState, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT player_key, name, account_name, player_id, user_id
FROM player_stat_state
WHERE online = 1`)
	if err != nil {
		return nil, fmt.Errorf("query online player states: %w", err)
	}
	defer rows.Close()

	players := make(map[string]storedPlayerState)
	for rows.Next() {
		var key string
		var player storedPlayerState
		if err := rows.Scan(&key, &player.name, &player.accountName, &player.playerID, &player.userID); err != nil {
			return nil, fmt.Errorf("scan online player state: %w", err)
		}
		players[key] = player
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate online player states: %w", err)
	}
	return players, nil
}

func upsertPlayerState(
	ctx context.Context,
	tx *sql.Tx,
	key string,
	recordedAtMS int64,
	name string,
	accountName string,
	playerID string,
	userID string,
	online bool,
) error {
	if _, err := tx.ExecContext(ctx, `
INSERT INTO player_stat_state (
	player_key, recorded_at_ms, name, account_name, player_id, user_id, online
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(player_key) DO UPDATE SET
	recorded_at_ms = excluded.recorded_at_ms,
	name = excluded.name,
	account_name = excluded.account_name,
	player_id = excluded.player_id,
	user_id = excluded.user_id,
	online = excluded.online`,
		key,
		recordedAtMS,
		name,
		accountName,
		playerID,
		userID,
		online,
	); err != nil {
		return fmt.Errorf("update player state: %w", err)
	}
	return nil
}

func playerKey(player domain.Player) string {
	switch {
	case player.UserID != "":
		return "user:" + player.UserID
	case player.AccountName != "":
		return "account:" + player.AccountName
	case player.PlayerID != "":
		return "player:" + player.PlayerID
	case player.Name != "":
		return "name:" + player.Name
	default:
		return ""
	}
}

func (s *Store) RecentMetrics(ctx context.Context, since time.Time, limit int) ([]domain.Metrics, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT recorded_at, server_fps, current_players, server_frame_time,
	max_players, uptime_seconds, base_camps, world_days
FROM metric_samples
WHERE recorded_at >= ?
ORDER BY recorded_at DESC
LIMIT ?`, since.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("query recent metrics: %w", err)
	}
	defer rows.Close()

	var samples []domain.Metrics
	for rows.Next() {
		var recordedAt int64
		var sample domain.Metrics
		if err := rows.Scan(
			&recordedAt,
			&sample.ServerFPS,
			&sample.CurrentPlayerNum,
			&sample.ServerFrameTime,
			&sample.MaxPlayerNum,
			&sample.Uptime,
			&sample.BaseCampNum,
			&sample.Days,
		); err != nil {
			return nil, fmt.Errorf("scan recent metrics: %w", err)
		}
		sample.RecordedAt = time.Unix(recordedAt, 0)
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent metrics: %w", err)
	}
	return samples, nil
}

func (s *Store) DeleteHistoryBefore(ctx context.Context, cutoff time.Time) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history cleanup: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, "DELETE FROM metric_samples WHERE recorded_at < ?", cutoff.Unix()); err != nil {
		return fmt.Errorf("delete old metrics: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM player_stat_samples WHERE recorded_at_ms < ?", cutoff.UnixMilli()); err != nil {
		return fmt.Errorf("delete old player stats: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
DELETE FROM player_stat_state
WHERE online = 0 AND recorded_at_ms < ?`, cutoff.UnixMilli()); err != nil {
		return fmt.Errorf("delete old player states: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit history cleanup: %w", err)
	}
	return nil
}
