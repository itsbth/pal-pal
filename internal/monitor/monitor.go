package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

type API interface {
	Info(context.Context) (domain.ServerInfo, error)
	Players(context.Context) ([]domain.Player, error)
	Metrics(context.Context) (domain.Metrics, error)
	GameData(context.Context) (domain.GameData, error)
}

type HistoryStore interface {
	RecordMetrics(context.Context, domain.Metrics) error
	RecordPlayerStats(context.Context, []domain.Player, time.Time) error
	DeleteHistoryBefore(context.Context, time.Time) error
}

type Monitor struct {
	api              API
	store            HistoryStore
	interval         time.Duration
	retention        time.Duration
	gameDataEnabled  bool
	gameDataInterval time.Duration
	log              *slog.Logger

	mu       sync.RWMutex
	snapshot domain.Snapshot
}

type Config struct {
	PollInterval     time.Duration
	HistoryRetention time.Duration
	GameDataEnabled  bool
	GameDataInterval time.Duration
}

func New(api API, store HistoryStore, cfg Config, log *slog.Logger) *Monitor {
	return &Monitor{
		api:              api,
		store:            store,
		interval:         cfg.PollInterval,
		retention:        cfg.HistoryRetention,
		gameDataEnabled:  cfg.GameDataEnabled,
		gameDataInterval: cfg.GameDataInterval,
		log:              log,
		snapshot: domain.Snapshot{
			GameDataEnabled: cfg.GameDataEnabled,
		},
	}
}

func (m *Monitor) Snapshot() domain.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := m.snapshot
	snapshot.Players = append([]domain.Player(nil), snapshot.Players...)
	snapshot.GameData.Actors = append([]domain.WorldActor(nil), snapshot.GameData.Actors...)
	return snapshot
}

func (m *Monitor) Run(ctx context.Context) error {
	m.poll(ctx)
	if m.gameDataEnabled {
		m.pollGameData(ctx)
	}

	pollTicker := time.NewTicker(m.interval)
	defer pollTicker.Stop()

	cleanupTicker := time.NewTicker(time.Hour)
	defer cleanupTicker.Stop()

	var gameDataC <-chan time.Time
	var gameDataTicker *time.Ticker
	if m.gameDataEnabled {
		gameDataTicker = time.NewTicker(m.gameDataInterval)
		gameDataC = gameDataTicker.C
		defer gameDataTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			m.poll(ctx)
		case <-gameDataC:
			m.pollGameData(ctx)
		case <-cleanupTicker.C:
			cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := m.store.DeleteHistoryBefore(cleanupCtx, time.Now().Add(-m.retention))
			cancel()
			if err != nil {
				m.log.Error("clean history", "error", err)
			}
		}
	}
}

func (m *Monitor) pollGameData(ctx context.Context) {
	gameData, err := m.api.GameData(ctx)
	now := time.Now()

	m.mu.Lock()
	if err == nil {
		gameData.RecordedAt = now
		m.snapshot.GameData = gameData
		m.snapshot.GameDataError = ""
	} else {
		m.snapshot.GameDataError = err.Error()
	}
	m.mu.Unlock()

	if err != nil {
		m.log.Warn("game data poll failed", "error", err)
	}
}

func (m *Monitor) Refresh(ctx context.Context) {
	m.poll(ctx)
}

func (m *Monitor) poll(ctx context.Context) {
	info, infoErr := m.api.Info(ctx)
	players, playersErr := m.api.Players(ctx)
	metrics, metricsErr := m.api.Metrics(ctx)
	now := time.Now()

	metrics.RecordedAt = now
	var errs []error
	if infoErr != nil {
		errs = append(errs, fmt.Errorf("server info: %w", infoErr))
	}
	if playersErr != nil {
		errs = append(errs, fmt.Errorf("players: %w", playersErr))
	} else if err := m.store.RecordPlayerStats(ctx, players, now); err != nil {
		errs = append(errs, err)
	}
	if metricsErr != nil {
		errs = append(errs, fmt.Errorf("metrics: %w", metricsErr))
	} else if err := m.store.RecordMetrics(ctx, metrics); err != nil {
		errs = append(errs, err)
	}

	m.mu.Lock()
	if infoErr == nil {
		m.snapshot.Info = info
	}
	if playersErr == nil {
		m.snapshot.Players = players
	}
	if metricsErr == nil {
		m.snapshot.Metrics = metrics
	}
	if len(errs) == 0 {
		m.snapshot.UpdatedAt = now
		m.snapshot.LastError = ""
	} else {
		m.snapshot.LastError = joinErrors(errs)
	}
	m.mu.Unlock()

	if len(errs) > 0 {
		m.log.Warn("monitor poll incomplete", "error", errors.Join(errs...))
	}
}

func joinErrors(errs []error) string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}
