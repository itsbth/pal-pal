package monitor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

type fakeAPI struct {
	gameData      domain.GameData
	gameDataErr   error
	gameDataCalls int
	players       []domain.Player
	playersErr    error
}

func (f *fakeAPI) Info(context.Context) (domain.ServerInfo, error) {
	return domain.ServerInfo{}, nil
}

func (f *fakeAPI) Players(context.Context) ([]domain.Player, error) {
	return f.players, f.playersErr
}

func (f *fakeAPI) Metrics(context.Context) (domain.Metrics, error) {
	return domain.Metrics{}, nil
}

func (f *fakeAPI) GameData(context.Context) (domain.GameData, error) {
	f.gameDataCalls++
	return f.gameData, f.gameDataErr
}

type fakeHistoryStore struct {
	playerSnapshots [][]domain.Player
}

func (*fakeHistoryStore) RecordMetrics(context.Context, domain.Metrics) error {
	return nil
}

func (f *fakeHistoryStore) RecordPlayerStats(_ context.Context, players []domain.Player, _ time.Time) error {
	f.playerSnapshots = append(f.playerSnapshots, append([]domain.Player(nil), players...))
	return nil
}

func (*fakeHistoryStore) DeleteHistoryBefore(context.Context, time.Time) error {
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunSkipsGameDataWhenDisabled(t *testing.T) {
	api := &fakeAPI{}
	monitor := New(api, &fakeHistoryStore{}, Config{
		PollInterval:     time.Minute,
		HistoryRetention: time.Hour,
		GameDataInterval: time.Minute,
	}, testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := monitor.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v", err)
	}
	if api.gameDataCalls != 0 {
		t.Fatalf("GameData() calls = %d", api.gameDataCalls)
	}
	if monitor.Snapshot().GameDataEnabled {
		t.Fatal("GameDataEnabled = true")
	}
}

func TestGameDataFailureIsIsolated(t *testing.T) {
	api := &fakeAPI{
		gameData: domain.GameData{
			InGameTime: "14:30",
			Actors:     []domain.WorldActor{{Type: "PalBox"}},
		},
	}
	monitor := New(api, &fakeHistoryStore{}, Config{
		PollInterval:     time.Minute,
		HistoryRetention: time.Hour,
		GameDataEnabled:  true,
		GameDataInterval: time.Minute,
	}, testLogger())

	monitor.pollGameData(context.Background())
	first := monitor.Snapshot()
	if first.GameData.InGameTime != "14:30" || first.GameData.RecordedAt.IsZero() {
		t.Fatalf("first game data = %#v", first.GameData)
	}

	api.gameDataErr = errors.New("endpoint disabled")
	monitor.pollGameData(context.Background())
	second := monitor.Snapshot()
	if second.GameDataError != "endpoint disabled" {
		t.Fatalf("GameDataError = %q", second.GameDataError)
	}
	if second.GameData.InGameTime != "14:30" {
		t.Fatalf("stale game data was discarded: %#v", second.GameData)
	}
	if second.LastError != "" {
		t.Fatalf("LastError = %q", second.LastError)
	}
}

func TestPollRecordsPlayersOnlyAfterSuccessfulPlayerPoll(t *testing.T) {
	api := &fakeAPI{
		players: []domain.Player{{Name: "Moss", UserID: "steam_1", Level: 12}},
	}
	history := &fakeHistoryStore{}
	monitor := New(api, history, Config{
		PollInterval:     time.Minute,
		HistoryRetention: time.Hour,
	}, testLogger())

	monitor.poll(context.Background())
	if len(history.playerSnapshots) != 1 || len(history.playerSnapshots[0]) != 1 {
		t.Fatalf("player snapshots = %#v", history.playerSnapshots)
	}

	api.playersErr = errors.New("players unavailable")
	api.players = nil
	monitor.poll(context.Background())
	if len(history.playerSnapshots) != 1 {
		t.Fatalf("failed poll recorded a snapshot: %#v", history.playerSnapshots)
	}
}
