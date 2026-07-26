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
}

type MetricsStore interface {
	RecordMetrics(context.Context, domain.Metrics) error
	DeleteMetricsBefore(context.Context, time.Time) error
}

type Monitor struct {
	api       API
	store     MetricsStore
	interval  time.Duration
	retention time.Duration
	log       *slog.Logger

	mu       sync.RWMutex
	snapshot domain.Snapshot
}

func New(api API, store MetricsStore, interval, retention time.Duration, log *slog.Logger) *Monitor {
	return &Monitor{
		api:       api,
		store:     store,
		interval:  interval,
		retention: retention,
		log:       log,
	}
}

func (m *Monitor) Snapshot() domain.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := m.snapshot
	snapshot.Players = append([]domain.Player(nil), snapshot.Players...)
	return snapshot
}

func (m *Monitor) Run(ctx context.Context) error {
	m.poll(ctx)

	pollTicker := time.NewTicker(m.interval)
	defer pollTicker.Stop()

	cleanupTicker := time.NewTicker(time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			m.poll(ctx)
		case <-cleanupTicker.C:
			cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := m.store.DeleteMetricsBefore(cleanupCtx, time.Now().Add(-m.retention))
			cancel()
			if err != nil {
				m.log.Error("clean metric history", "error", err)
			}
		}
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
