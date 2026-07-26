package web

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

type pageData struct {
	Title        string
	Active       string
	IsAdmin      bool
	CanSignOut   bool
	PublicRead   bool
	CSRFToken    string
	Error        string
	Flash        string
	Snapshot     domain.Snapshot
	Players      []playerView
	Markers      []mapMarker
	WorldSummary worldSummary
	Settings     []settingView
	Metrics      []domain.Metrics
	FPSPoints    string
	PlayerPoints string
	ChartStart   string
	ChartEnd     string
	MapImageURL  string
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
}

type settingView struct {
	Name  string
	Value string
}

type worldSummary struct {
	Actors   int
	BasePals int
	WildPals int
	PalBoxes int
}

const (
	mapCalibrationWorldX = -357558.0
	mapCalibrationWorldY = 268878.0
	mapCalibrationSVGX   = 68.62
	mapCalibrationSVGY   = 48.76
	mapWorldSpan         = 1447840.0
)

func summarizeWorld(actors []domain.WorldActor) worldSummary {
	summary := worldSummary{Actors: len(actors)}
	for _, actor := range actors {
		if actor.Type == "PalBox" {
			summary.PalBoxes++
		}
		switch actor.UnitType {
		case "BaseCampPal":
			summary.BasePals++
		case "WildPal":
			summary.WildPals++
		}
	}
	return summary
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

	result := make([]mapMarker, 0, len(players))
	for _, player := range players {
		x, y := mapPosition(player.LocationX, player.LocationY)
		result = append(result, mapMarker{
			Name: player.Name,
			X:    x,
			Y:    y,
		})
	}
	return result
}

func mapPosition(worldX, worldY float64) (float64, float64) {
	// Palworld's world axes are swapped on the map: world Y runs left-to-right,
	// while increasing world X runs bottom-to-top.
	x := mapCalibrationSVGX + ((worldY - mapCalibrationWorldY) / mapWorldSpan * 100)
	y := mapCalibrationSVGY - ((worldX - mapCalibrationWorldX) / mapWorldSpan * 100)
	return x, y
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
	for index, sample := range slices.Backward(samples) {
		position := len(samples) - 1 - index
		x := left
		if len(samples) > 1 {
			x += (float64(position) / float64(len(samples)-1)) * width
		}
		y := top + height - (value(sample)/maxValue)*height
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
