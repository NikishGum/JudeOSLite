package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>PRIDE</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app-123.js"), []byte("console.log('ready')"), 0600); err != nil {
		t.Fatal(err)
	}
	ready := true
	h, err := NewHandler(dir, func(context.Context) error {
		if !ready {
			return errors.New("secret connection details")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path   string
		status         int
		content, cache string
	}{
		{"GET", "/api/healthz", 200, "application/json; charset=utf-8", "no-store"},
		{"GET", "/api/readyz", 200, "application/json; charset=utf-8", "no-store"},
		{"GET", "/api/missing", 404, "application/json; charset=utf-8", "no-store"},
		{"POST", "/api/readyz", 405, "application/json; charset=utf-8", "no-store"},
		{"GET", "/", 200, "text/html; charset=utf-8", "no-cache"},
		{"GET", "/future-route", 200, "text/html; charset=utf-8", "no-cache"},
		{"GET", "/missing.js", 404, "application/json; charset=utf-8", "no-store"},
		{"GET", "/assets/app-123.js", 200, "text/javascript; charset=utf-8", "public, max-age=31536000, immutable"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			r := httptest.NewRecorder()
			h.ServeHTTP(r, httptest.NewRequest(tc.method, tc.path, nil))
			if r.Code != tc.status || r.Header().Get("Content-Type") != tc.content || r.Header().Get("Cache-Control") != tc.cache {
				t.Fatalf("unexpected response: %d %v", r.Code, r.Header())
			}
		})
	}
	ready = false
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/api/readyz", nil))
	var body ErrorResponse
	if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if r.Code != 503 || body.Error.Code != "not_ready" || body.Error.Message != "Система временно недоступна." {
		t.Fatalf("unsafe readiness error: %s", r.Body.String())
	}
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/api/healthz", nil))
	if r.Code != 200 {
		t.Fatal("liveness must survive database unavailability")
	}
}

func TestMissingConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := LoadConfig(true); err == nil {
		t.Fatal("missing database config accepted")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("HTTP_ADDR", ":invalid")
	if _, err := LoadConfig(true); err == nil {
		t.Fatal("invalid port accepted")
	}
	t.Setenv("HTTP_ADDR", "")
	if _, err := LoadConfig(true); err != nil {
		t.Fatal(err)
	}
	if _, err := NewHandler(t.TempDir(), nil); err == nil {
		t.Fatal("missing frontend accepted")
	}
}
