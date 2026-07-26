package web

import (
	"testing"

	"github.com/itsbth/pal-pal/internal/domain"
)

func TestPlayersForViewRedactsIPForViewer(t *testing.T) {
	players := []domain.Player{{
		Name:   "Moss",
		UserID: "steam_1",
		IP:     "192.0.2.10",
	}}

	viewerRows := playersForView(players, false)
	if viewerRows[0].IP != "" {
		t.Fatalf("viewer IP = %q, want redacted", viewerRows[0].IP)
	}

	adminRows := playersForView(players, true)
	if adminRows[0].IP != "192.0.2.10" {
		t.Fatalf("admin IP = %q", adminRows[0].IP)
	}
}

func TestSummarizeWorld(t *testing.T) {
	actors := []domain.WorldActor{
		{Type: "Character", UnitType: "Player"},
		{Type: "Character", UnitType: "BaseCampPal"},
		{Type: "Character", UnitType: "BaseCampPal"},
		{Type: "Character", UnitType: "WildPal"},
		{Type: "PalBox"},
	}

	summary := summarizeWorld(actors)
	if summary.Actors != 5 || summary.BasePals != 2 || summary.WildPals != 1 || summary.PalBoxes != 1 {
		t.Fatalf("summarizeWorld() = %#v", summary)
	}
}

func TestMarkersUseFixedMapCalibration(t *testing.T) {
	anchor := domain.Player{
		Name:      "Moss",
		LocationX: -357558,
		LocationY: 268878,
	}

	marker := markersForView([]domain.Player{anchor})[0]
	if marker.X != 68.62 || marker.Y != 48.76 {
		t.Fatalf("anchor marker = (%.4f, %.4f), want (68.62, 48.76)", marker.X, marker.Y)
	}

	markerWithCompany := markersForView([]domain.Player{
		anchor,
		{Name: "Fox", LocationX: 100000, LocationY: -100000},
	})[0]
	if markerWithCompany.X != marker.X || markerWithCompany.Y != marker.Y {
		t.Fatalf(
			"anchor moved from (%.4f, %.4f) to (%.4f, %.4f) when another player joined",
			marker.X,
			marker.Y,
			markerWithCompany.X,
			markerWithCompany.Y,
		)
	}
}
