package web

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

func TestOverviewRendersOptionalWorldSnapshot(t *testing.T) {
	renderer, err := newRenderer()
	if err != nil {
		t.Fatalf("newRenderer() error = %v", err)
	}

	var output bytes.Buffer
	err = renderer.component(&output, "overview", pageData{
		Snapshot: domain.Snapshot{
			GameDataEnabled: true,
			GameData: domain.GameData{
				InGameTime: "14:30",
				InGameDays: 5,
				FPS:        59.6,
				AverageFPS: 58.9,
				RecordedAt: time.Date(2026, 7, 26, 20, 16, 45, 0, time.UTC),
			},
		},
		WorldSummary: worldSummary{
			Actors:   12,
			BasePals: 4,
			WildPals: 6,
			PalBoxes: 1,
		},
	})
	if err != nil {
		t.Fatalf("component() error = %v", err)
	}

	body := output.String()
	for _, expected := range []string{"World snapshot", "14:30", "day 5", "Base Pals", ">4<"} {
		if !strings.Contains(body, expected) {
			t.Errorf("overview does not contain %q", expected)
		}
	}
}

func TestOverviewOmitsWorldSnapshotWhenDisabled(t *testing.T) {
	renderer, err := newRenderer()
	if err != nil {
		t.Fatalf("newRenderer() error = %v", err)
	}

	var output bytes.Buffer
	if err := renderer.component(&output, "overview", pageData{}); err != nil {
		t.Fatalf("component() error = %v", err)
	}
	if strings.Contains(output.String(), "World snapshot") {
		t.Fatal("disabled overview contains world snapshot")
	}
}
