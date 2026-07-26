package web

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

type pageData struct {
	Title        string
	Active       string
	Role         string
	IsAdmin      bool
	CanSignOut   bool
	PublicRead   bool
	CSRFToken    string
	Error        string
	Flash        string
	Snapshot     domain.Snapshot
	Players      []playerView
	Markers      []mapMarker
	Settings     []settingView
	Metrics      []domain.Metrics
	FPSPoints    string
	PlayerPoints string
	ChartStart   string
	ChartEnd     string
}

type playerView struct {
	Name          string
	AccountName   string
	UserID        string
	IP            string
	Ping          float64
	LocationX     float64
	LocationY     float64
	Level         int
	BuildingCount int
}

type mapMarker struct {
	Name string
	X    float64
	Y    float64
	RawX float64
	RawY float64
}

type settingView struct {
	Name  string
	Value string
}

func playersForView(players []domain.Player, isAdmin bool) []playerView {
	result := make([]playerView, 0, len(players))
	for _, player := range players {
		ip := ""
		if isAdmin {
			ip = player.IP
		}
		result = append(result, playerView{
			Name:          player.Name,
			AccountName:   player.AccountName,
			UserID:        player.UserID,
			IP:            ip,
			Ping:          player.Ping,
			LocationX:     player.LocationX,
			LocationY:     player.LocationY,
			Level:         player.Level,
			BuildingCount: player.BuildingCount,
		})
	}
	return result
}

func markersForView(players []domain.Player) []mapMarker {
	if len(players) == 0 {
		return nil
	}

	minX, maxX := players[0].LocationX, players[0].LocationX
	minY, maxY := players[0].LocationY, players[0].LocationY
	for _, player := range players[1:] {
		minX = math.Min(minX, player.LocationX)
		maxX = math.Max(maxX, player.LocationX)
		minY = math.Min(minY, player.LocationY)
		maxY = math.Max(maxY, player.LocationY)
	}

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX == 0 {
		rangeX = 1
	}
	if rangeY == 0 {
		rangeY = 1
	}

	result := make([]mapMarker, 0, len(players))
	for _, player := range players {
		result = append(result, mapMarker{
			Name: player.Name,
			X:    7 + ((player.LocationX-minX)/rangeX)*86,
			Y:    93 - ((player.LocationY-minY)/rangeY)*86,
			RawX: player.LocationX,
			RawY: player.LocationY,
		})
	}
	return result
}

func settingsForView(settings map[string]any) []settingView {
	result := make([]settingView, 0, len(settings))
	for name, value := range settings {
		result = append(result, settingView{Name: name, Value: fmt.Sprint(value)})
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func graphPoints(samples []domain.Metrics, value func(domain.Metrics) float64) string {
	if len(samples) == 0 {
		return ""
	}

	maxValue := 1.0
	for _, sample := range samples {
		maxValue = math.Max(maxValue, value(sample))
	}

	const (
		left   = 32.0
		top    = 16.0
		width  = 736.0
		height = 168.0
	)

	var points strings.Builder
	for index := len(samples) - 1; index >= 0; index-- {
		position := len(samples) - 1 - index
		x := left
		if len(samples) > 1 {
			x += (float64(position) / float64(len(samples)-1)) * width
		}
		y := top + height - (value(samples[index])/maxValue)*height
		if points.Len() > 0 {
			points.WriteByte(' ')
		}
		fmt.Fprintf(&points, "%.1f,%.1f", x, y)
	}
	return points.String()
}

func metricRange(samples []domain.Metrics) (string, string) {
	if len(samples) == 0 {
		return "", ""
	}
	return samples[len(samples)-1].RecordedAt.Format(time.Kitchen), samples[0].RecordedAt.Format(time.Kitchen)
}
