package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

func TestRecordPlayerStatsIncludesOfflineTransitions(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	ctx := context.Background()
	start := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	players := []domain.Player{
		{
			Name:        "Moss",
			AccountName: "moss-account",
			PlayerID:    "player-1",
			UserID:      "steam-1",
			LocationX:   123.5,
			LocationY:   -42.25,
			Level:       12,
		},
		{
			Name:      "Lifmunk",
			PlayerID:  "player-2",
			UserID:    "steam-2",
			LocationX: 99,
			LocationY: 101,
			Level:     8,
		},
	}
	if err := database.RecordPlayerStats(ctx, players, start); err != nil {
		t.Fatalf("RecordPlayerStats(first) error = %v", err)
	}

	players[0].LocationX = 130
	players[0].Level = 13
	if err := database.RecordPlayerStats(ctx, players[:1], start.Add(15*time.Second)); err != nil {
		t.Fatalf("RecordPlayerStats(second) error = %v", err)
	}

	samples, err := database.RecentPlayerStats(ctx, start, 10)
	if err != nil {
		t.Fatalf("RecentPlayerStats() error = %v", err)
	}
	if len(samples) != 4 {
		t.Fatalf("len(samples) = %d, want 4: %#v", len(samples), samples)
	}

	var online, offline *domain.PlayerStat
	for index := range samples {
		sample := &samples[index]
		if sample.RecordedAt.Equal(start.Add(15*time.Second)) && sample.UserID == "steam-1" {
			online = sample
		}
		if sample.RecordedAt.Equal(start.Add(15*time.Second)) && sample.UserID == "steam-2" {
			offline = sample
		}
	}
	if online == nil || !online.Online || online.LocationX == nil || *online.LocationX != 130 || online.Level == nil || *online.Level != 13 {
		t.Fatalf("online sample = %#v", online)
	}
	if online.AccountName != "moss-account" {
		t.Fatalf("online AccountName = %q", online.AccountName)
	}
	if offline == nil || offline.Online || offline.LocationX != nil || offline.LocationY != nil || offline.Level != nil {
		t.Fatalf("offline sample = %#v", offline)
	}
}

func TestPlayerStatsUseFallbackIdentityAndDeduplicate(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	ctx := context.Background()
	recordedAt := time.Now()
	player := domain.Player{Name: "Moss", AccountName: "moss-account", Level: 4}
	if err := database.RecordPlayerStats(ctx, []domain.Player{player, player, {}}, recordedAt); err != nil {
		t.Fatalf("RecordPlayerStats() error = %v", err)
	}

	samples, err := database.RecentPlayerStats(ctx, recordedAt.Add(-time.Second), 10)
	if err != nil {
		t.Fatalf("RecentPlayerStats() error = %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("len(samples) = %d, want 1", len(samples))
	}
	if samples[0].PlayerKey != "account:moss-account" {
		t.Fatalf("PlayerKey = %q", samples[0].PlayerKey)
	}
}

func TestRecordPlayerStatsTracksOfflineTransitionAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx := context.Background()
	start := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	player := domain.Player{Name: "Moss", AccountName: "moss-account", UserID: "steam-1"}
	if err := database.RecordPlayerStats(ctx, []domain.Player{player}, start); err != nil {
		t.Fatalf("RecordPlayerStats(online) error = %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()
	if err := database.RecordPlayerStats(ctx, nil, start.Add(time.Minute)); err != nil {
		t.Fatalf("RecordPlayerStats(offline) error = %v", err)
	}

	samples, err := database.RecentPlayerStats(ctx, start.Add(30*time.Second), 10)
	if err != nil {
		t.Fatalf("RecentPlayerStats() error = %v", err)
	}
	if len(samples) != 1 || samples[0].Online || samples[0].AccountName != "moss-account" {
		t.Fatalf("offline samples = %#v", samples)
	}
}

func TestDeleteHistoryBeforeRemovesPlayerSamples(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	ctx := context.Background()
	start := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	player := domain.Player{Name: "Moss", UserID: "steam-1"}
	if err := database.RecordPlayerStats(ctx, []domain.Player{player}, start); err != nil {
		t.Fatalf("RecordPlayerStats(online) error = %v", err)
	}
	if err := database.RecordPlayerStats(ctx, nil, start.Add(time.Minute)); err != nil {
		t.Fatalf("RecordPlayerStats(offline) error = %v", err)
	}
	if err := database.DeleteHistoryBefore(ctx, start.Add(2*time.Minute)); err != nil {
		t.Fatalf("DeleteHistoryBefore() error = %v", err)
	}

	samples, err := database.RecentPlayerStats(ctx, time.Time{}, 10)
	if err != nil {
		t.Fatalf("RecentPlayerStats() error = %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("samples after cleanup = %#v", samples)
	}

	var stateCount int
	if err := database.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM player_stat_state").Scan(&stateCount); err != nil {
		t.Fatalf("count player states: %v", err)
	}
	if stateCount != 0 {
		t.Fatalf("state count = %d, want 0", stateCount)
	}
}
