package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapImageServesConfiguredFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.webp")
	if err := os.WriteFile(path, []byte("RIFF-test-webp"), 0o600); err != nil {
		t.Fatalf("write map image: %v", err)
	}

	server := &Server{mapImagePath: path}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/map-image", nil)
	response := httptest.NewRecorder()

	server.mapImage(response, request, session{})

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "image/webp") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "private, no-cache" {
		t.Fatalf("Cache-Control = %q", cacheControl)
	}
	if response.Body.String() != "RIFF-test-webp" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestMapImageReturnsNotFoundWhenUnavailable(t *testing.T) {
	server := &Server{mapImagePath: filepath.Join(t.TempDir(), "missing.webp")}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/map-image", nil)
	response := httptest.NewRecorder()

	server.mapImage(response, request, session{})

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if server.hasMapImage() {
		t.Fatal("hasMapImage() = true for missing file")
	}
}
