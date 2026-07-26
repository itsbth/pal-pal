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
	ON metric_samples(recorded_at DESC);`
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

func (s *Store) DeleteMetricsBefore(ctx context.Context, cutoff time.Time) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM metric_samples WHERE recorded_at < ?", cutoff.Unix()); err != nil {
		return fmt.Errorf("delete old metrics: %w", err)
	}
	return nil
}
