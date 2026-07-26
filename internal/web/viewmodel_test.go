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
