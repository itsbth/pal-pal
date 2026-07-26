package domain

import "time"

type ServerInfo struct {
	Version     string `json:"version"`
	ServerName  string `json:"servername"`
	Description string `json:"description"`
	WorldGUID   string `json:"worldguid"`
}

type Player struct {
	Name          string  `json:"name"`
	AccountName   string  `json:"accountName"`
	PlayerID      string  `json:"playerId"`
	UserID        string  `json:"userId"`
	IP            string  `json:"ip"`
	Ping          float64 `json:"ping"`
	LocationX     float64 `json:"location_x"`
	LocationY     float64 `json:"location_y"`
	Level         int     `json:"level"`
	BuildingCount int     `json:"building_count"`
}

// PlayerStat is a retained observation used to build player timelines.
// Position and level are nil while the player is offline.
type PlayerStat struct {
	PlayerKey   string
	Name        string
	AccountName string
	PlayerID    string
	UserID      string
	Online      bool
	LocationX   *float64
	LocationY   *float64
	Level       *int
	RecordedAt  time.Time
}

type Metrics struct {
	ServerFPS        int     `json:"serverfps"`
	CurrentPlayerNum int     `json:"currentplayernum"`
	ServerFrameTime  float64 `json:"serverframetime"`
	MaxPlayerNum     int     `json:"maxplayernum"`
	Uptime           int64   `json:"uptime"`
	BaseCampNum      int     `json:"basecampnum"`
	Days             int     `json:"days"`
	RecordedAt       time.Time
}

type WorldActor struct {
	Type     string `json:"Type"`
	UnitType string `json:"UnitType"`
}

type GameData struct {
	SnapshotTime string       `json:"Time"`
	FPS          float64      `json:"FPS"`
	AverageFPS   float64      `json:"AverageFPS"`
	InGameTime   string       `json:"InGameTime"`
	InGameDays   int          `json:"InGameDays"`
	Actors       []WorldActor `json:"ActorData"`
	RecordedAt   time.Time
}

type Snapshot struct {
	Info            ServerInfo
	Players         []Player
	Metrics         Metrics
	GameDataEnabled bool
	GameData        GameData
	GameDataError   string
	UpdatedAt       time.Time
	LastError       string
}
