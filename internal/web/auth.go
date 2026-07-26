package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const sessionCookieName = "palpal_session"

type role uint8

const (
	roleNone role = iota
	roleViewer
	roleAdmin
)

type session struct {
	Role      role
	CSRFToken string
	ExpiresAt time.Time
}

type sessions struct {
	mu             sync.Mutex
	entries        map[string]session
	publicRead     bool
	publicPassword string
	adminPassword  string
	secureCookies  bool
}

func newSessions(publicRead bool, publicPassword, adminPassword string, secureCookies bool) *sessions {
	return &sessions{
		entries:        make(map[string]session),
		publicRead:     publicRead,
		publicPassword: publicPassword,
		adminPassword:  adminPassword,
		secureCookies:  secureCookies,
	}
}

func (s *sessions) authenticate(password string) role {
	if constantTimeEqual(password, s.adminPassword) {
		return roleAdmin
	}
	if s.publicPassword != "" && constantTimeEqual(password, s.publicPassword) {
		return roleViewer
	}
	return roleNone
}

func (s *sessions) create(w http.ResponseWriter, access role) (session, error) {
	token, err := randomToken()
	if err != nil {
		return session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return session{}, err
	}

	value := session{
		Role:      access,
		CSRFToken: csrf,
		ExpiresAt: time.Now().Add(12 * time.Hour),
	}

	s.mu.Lock()
	s.entries[token] = value
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((12 * time.Hour).Seconds()),
	})
	return value, nil
}

func (s *sessions) get(r *http.Request) session {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		if s.publicRead {
			return session{Role: roleViewer}
		}
		return session{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.entries[cookie.Value]
	if !ok || time.Now().After(value.ExpiresAt) {
		delete(s.entries, cookie.Value)
		if s.publicRead {
			return session{Role: roleViewer}
		}
		return session{}
	}
	return value
}

func (s *sessions) destroy(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.mu.Lock()
		delete(s.entries, cookie.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func constantTimeEqual(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
