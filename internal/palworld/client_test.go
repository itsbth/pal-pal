package palworld

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlayers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "secret" {
			t.Errorf("BasicAuth = %q, %q, %v", username, password, ok)
		}
		if r.URL.Path != "/v1/api/players" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"players": []map[string]any{{"name": "Moss", "userId": "steam_1"}},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v1/api", "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	players, err := client.Players(context.Background())
	if err != nil {
		t.Fatalf("Players() error = %v", err)
	}
	if len(players) != 1 || players[0].Name != "Moss" {
		t.Fatalf("Players() = %#v", players)
	}
}

func TestNewClientAddsDefaultAPIPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/info" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"servername": "Default path"})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if info.ServerName != "Default path" {
		t.Fatalf("ServerName = %q", info.ServerName)
	}
}

func TestGameData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/game-data" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Time":       "2026-07-26 20:16:45",
			"FPS":        59.6,
			"AverageFPS": 58.9,
			"InGameTime": "14:30",
			"InGameDays": 5,
			"ActorData": []map[string]any{
				{"Type": "Character", "UnitType": "BaseCampPal", "ip": "192.0.2.1"},
				{"Type": "PalBox"},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	gameData, err := client.GameData(context.Background())
	if err != nil {
		t.Fatalf("GameData() error = %v", err)
	}
	if gameData.InGameTime != "14:30" || gameData.InGameDays != 5 {
		t.Fatalf("GameData() time = %q, day = %d", gameData.InGameTime, gameData.InGameDays)
	}
	if len(gameData.Actors) != 2 || gameData.Actors[0].UnitType != "BaseCampPal" {
		t.Fatalf("GameData() actors = %#v", gameData.Actors)
	}
}
