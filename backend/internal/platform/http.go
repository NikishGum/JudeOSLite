package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	var body ErrorResponse
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func NewHandler(staticDir string, ready func(context.Context) error) (http.Handler, error) {
	index := filepath.Join(staticDir, "index.html")
	if f, err := os.Stat(index); err != nil || f.IsDir() {
		return nil, fmt.Errorf("STATIC_DIR must contain a built index.html")
	}
	fs := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Метод не поддерживается.")
			return
		}
		switch r.URL.Path {
		case "/api/healthz":
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		case "/api/readyz":
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := ready(ctx); err != nil {
				WriteError(w, http.StatusServiceUnavailable, "not_ready", "Система временно недоступна.")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			WriteError(w, http.StatusNotFound, "not_found", "Адрес не найден.")
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		f, err := os.Stat(filepath.Join(staticDir, filepath.FromSlash(strings.TrimPrefix(clean, "/"))))
		if err == nil && !f.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fs.ServeHTTP(w, r)
			return
		}
		if path.Ext(clean) != "" {
			WriteError(w, http.StatusNotFound, "not_found", "Файл не найден.")
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	}), nil
}
