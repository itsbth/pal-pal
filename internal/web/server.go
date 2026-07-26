package web

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/itsbth/pal-pal/internal/domain"
)

type API interface {
	Kick(context.Context, string, string) error
	Ban(context.Context, string, string) error
	Unban(context.Context, string) error
	Settings(context.Context) (map[string]any, error)
}

type SnapshotSource interface {
	Snapshot() domain.Snapshot
	Refresh(context.Context)
}

type MetricHistory interface {
	RecentMetrics(context.Context, time.Time, int) ([]domain.Metrics, error)
}

type Config struct {
	PublicRead     bool
	PublicPassword string
	AdminPassword  string
	SecureCookies  bool
	MapImagePath   string
}

type Server struct {
	api          API
	monitor      SnapshotSource
	store        MetricHistory
	log          *slog.Logger
	render       *renderer
	sessions     *sessions
	publicRead   bool
	mapImagePath string
	handler      http.Handler
}

func New(
	api API,
	monitor SnapshotSource,
	store MetricHistory,
	cfg Config,
	log *slog.Logger,
) (*Server, error) {
	render, err := newRenderer()
	if err != nil {
		return nil, err
	}

	server := &Server{
		api:          api,
		monitor:      monitor,
		store:        store,
		log:          log,
		render:       render,
		sessions:     newSessions(cfg.PublicRead, cfg.PublicPassword, cfg.AdminPassword, cfg.SecureCookies),
		publicRead:   cfg.PublicRead,
		mapImagePath: cfg.MapImagePath,
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(render.static)))
	mux.HandleFunc("GET /login", server.loginPage)
	mux.HandleFunc("POST /login", server.login)
	mux.HandleFunc("POST /logout", server.logout)

	mux.HandleFunc("GET /", server.withViewer(server.dashboardPage))
	mux.HandleFunc("GET /players", server.withViewer(server.playersPage))
	mux.HandleFunc("GET /map", server.withViewer(server.mapPage))
	mux.HandleFunc("GET /map-image", server.withViewer(server.mapImage))
	mux.HandleFunc("GET /metrics", server.withViewer(server.metricsPage))
	mux.HandleFunc("GET /settings", server.withAdmin(server.settingsPage))

	mux.HandleFunc("GET /components/overview", server.withViewer(server.overviewComponent))
	mux.HandleFunc("GET /components/players", server.withViewer(server.playersComponent))
	mux.HandleFunc("GET /components/map", server.withViewer(server.mapComponent))
	mux.HandleFunc("GET /components/metrics", server.withViewer(server.metricsComponent))

	mux.HandleFunc("POST /admin/players/kick", server.withAdmin(server.kickPlayer))
	mux.HandleFunc("POST /admin/players/ban", server.withAdmin(server.banPlayer))
	mux.HandleFunc("POST /admin/players/unban", server.withAdmin(server.unbanPlayer))

	server.handler = server.securityHeaders(mux)
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.sessions.get(r).Role == roleAdmin {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderPage(w, "login", pageData{Title: "Sign in", Active: "login", PublicRead: s.publicRead})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderPageStatus(w, "login", pageData{Title: "Sign in", Active: "login", Error: "Could not read the sign-in form."}, http.StatusBadRequest)
		return
	}

	access := s.sessions.authenticate(r.FormValue("password"))
	if access == roleNone {
		time.Sleep(200 * time.Millisecond)
		s.renderPageStatus(w, "login", pageData{
			Title:      "Sign in",
			Active:     "login",
			PublicRead: s.publicRead,
			Error:      "That password was not recognized.",
		}, http.StatusUnauthorized)
		return
	}
	if _, err := s.sessions.create(w, access); err != nil {
		s.log.Error("create session", "error", err)
		http.Error(w, "Could not start a session.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	value := s.sessions.get(r)
	if value.CSRFToken != "" && !validCSRF(value.CSRFToken, r.FormValue("csrf_token")) {
		http.Error(w, "Invalid form token.", http.StatusForbidden)
		return
	}
	s.sessions.destroy(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) dashboardPage(w http.ResponseWriter, _ *http.Request, current session) {
	s.renderPage(w, "dashboard", s.baseData("Overview", "overview", current))
}

func (s *Server) playersPage(w http.ResponseWriter, _ *http.Request, current session) {
	s.renderPage(w, "players", s.baseData("Players", "players", current))
}

func (s *Server) mapPage(w http.ResponseWriter, _ *http.Request, current session) {
	s.renderPage(w, "map", s.baseData("Map", "map", current))
}

func (s *Server) metricsPage(w http.ResponseWriter, _ *http.Request, current session) {
	s.renderPage(w, "metrics", s.baseData("Metrics", "metrics", current))
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request, current session) {
	data := s.baseData("Server settings", "settings", current)
	settings, err := s.api.Settings(r.Context())
	if err != nil {
		data.Error = "Settings are currently unavailable: " + err.Error()
	} else {
		data.Settings = settingsForView(settings)
	}
	s.renderPage(w, "settings", data)
}

func (s *Server) overviewComponent(w http.ResponseWriter, _ *http.Request, current session) {
	data := s.baseData("", "", current)
	data.Snapshot = s.monitor.Snapshot()
	data.WorldSummary = summarizeWorld(data.Snapshot.GameData.Actors)
	s.renderComponent(w, "overview", data)
}

func (s *Server) playersComponent(w http.ResponseWriter, r *http.Request, current session) {
	data := s.baseData("", "", current)
	data.Snapshot = s.monitor.Snapshot()
	data.Players = playersForView(data.Snapshot.Players, data.IsAdmin)
	data.Flash = safeFlash(r.URL.Query().Get("flash"))
	s.renderComponent(w, "players", data)
}

func (s *Server) mapComponent(w http.ResponseWriter, _ *http.Request, current session) {
	data := s.baseData("", "", current)
	data.Snapshot = s.monitor.Snapshot()
	data.Markers = markersForView(data.Snapshot.Players)
	if s.hasMapImage() {
		data.MapImageURL = "/map-image"
	}
	s.renderComponent(w, "map", data)
}

func (s *Server) mapImage(w http.ResponseWriter, r *http.Request, _ session) {
	file, err := os.Open(s.mapImagePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (s *Server) hasMapImage() bool {
	info, err := os.Stat(s.mapImagePath)
	return err == nil && info.Mode().IsRegular()
}

func (s *Server) metricsComponent(w http.ResponseWriter, r *http.Request, current session) {
	data := s.baseData("", "", current)
	data.Snapshot = s.monitor.Snapshot()
	samples, err := s.store.RecentMetrics(r.Context(), time.Now().Add(-24*time.Hour), 5760)
	if err != nil {
		data.Error = "Metric history is currently unavailable: " + err.Error()
	} else {
		data.Metrics = samples
		data.FPSPoints = graphPoints(samples, func(sample domain.Metrics) float64 {
			return float64(sample.ServerFPS)
		})
		data.PlayerPoints = graphPoints(samples, func(sample domain.Metrics) float64 {
			return float64(sample.CurrentPlayerNum)
		})
		data.ChartStart, data.ChartEnd = metricRange(samples)
	}
	s.renderComponent(w, "metrics", data)
}

func (s *Server) kickPlayer(w http.ResponseWriter, r *http.Request, current session) {
	s.playerAction(w, r, current, "kicked", s.api.Kick)
}

func (s *Server) banPlayer(w http.ResponseWriter, r *http.Request, current session) {
	s.playerAction(w, r, current, "banned", s.api.Ban)
}

func (s *Server) unbanPlayer(w http.ResponseWriter, r *http.Request, current session) {
	if !s.checkCSRF(w, r, current) {
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" || len(userID) > 128 {
		http.Error(w, "A valid user ID is required.", http.StatusBadRequest)
		return
	}
	if err := s.api.Unban(r.Context(), userID); err != nil {
		s.renderActionError(w, r, current, "Could not unban player: "+err.Error())
		return
	}
	s.refresh(r.Context())
	s.renderPlayersAfterAction(w, r, current, "Player unbanned.")
}

func (s *Server) playerAction(
	w http.ResponseWriter,
	r *http.Request,
	current session,
	pastTense string,
	action func(context.Context, string, string) error,
) {
	if !s.checkCSRF(w, r, current) {
		return
	}
	userID := strings.TrimSpace(r.FormValue("user_id"))
	message := strings.TrimSpace(r.FormValue("message"))
	if userID == "" || len(userID) > 128 || len(message) > 500 {
		http.Error(w, "The player ID or message is invalid.", http.StatusBadRequest)
		return
	}
	if err := action(r.Context(), userID, message); err != nil {
		s.renderActionError(w, r, current, "Could not update player: "+err.Error())
		return
	}
	s.refresh(r.Context())
	s.renderPlayersAfterAction(w, r, current, "Player "+pastTense+".")
}

func (s *Server) renderPlayersAfterAction(w http.ResponseWriter, r *http.Request, current session, flash string) {
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/players?flash="+url.QueryEscape(flash), http.StatusSeeOther)
		return
	}
	data := s.baseData("", "", current)
	data.Snapshot = s.monitor.Snapshot()
	data.Players = playersForView(data.Snapshot.Players, true)
	data.Flash = flash
	s.renderComponent(w, "players", data)
}

func (s *Server) renderActionError(w http.ResponseWriter, _ *http.Request, current session, message string) {
	data := s.baseData("", "", current)
	data.Snapshot = s.monitor.Snapshot()
	data.Players = playersForView(data.Snapshot.Players, true)
	data.Error = message
	w.WriteHeader(http.StatusBadGateway)
	s.renderComponent(w, "players", data)
}

func (s *Server) refresh(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	s.monitor.Refresh(ctx)
}

func (s *Server) checkCSRF(w http.ResponseWriter, r *http.Request, current session) bool {
	if current.CSRFToken == "" || !validCSRF(current.CSRFToken, r.FormValue("csrf_token")) {
		http.Error(w, "Invalid form token.", http.StatusForbidden)
		return false
	}
	return true
}

func validCSRF(expected, actual string) bool {
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func (s *Server) baseData(title, active string, current session) pageData {
	return pageData{
		Title:      title,
		Active:     active,
		IsAdmin:    current.Role == roleAdmin,
		CanSignOut: current.CSRFToken != "",
		PublicRead: s.publicRead,
		CSRFToken:  current.CSRFToken,
	}
}

func (s *Server) withViewer(next func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current := s.sessions.get(r)
		if current.Role < roleViewer {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r, current)
	}
}

func (s *Server) withAdmin(next func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return s.withViewer(func(w http.ResponseWriter, r *http.Request, current session) {
		if current.Role != roleAdmin {
			http.Error(w, "Administrator access required.", http.StatusForbidden)
			return
		}
		next(w, r, current)
	})
}

func (s *Server) renderPage(w http.ResponseWriter, name string, data pageData) {
	s.renderPageStatus(w, name, data, http.StatusOK)
}

func (s *Server) renderPageStatus(w http.ResponseWriter, name string, data pageData, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.render.page(w, name, data); err != nil {
		s.log.Error("render page", "template", name, "error", err)
	}
}

func (s *Server) renderComponent(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.render.component(w, name, data); err != nil {
		s.log.Error("render component", "template", name, "error", err)
	}
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://unpkg.com; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func safeFlash(value string) string {
	if len(value) > 120 {
		return ""
	}
	return value
}
