package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionLifetime = 12 * time.Hour

type Config struct {
	Origin        string
	InsecureLocal bool
}

func (c Config) Validate() error {
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("APP_ORIGIN must be an exact http(s) origin without a path")
	}
	if c.InsecureLocal && (u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1")) {
		return errors.New("AUTH_INSECURE_LOCAL is allowed only for an HTTP loopback APP_ORIGIN")
	}
	if !c.InsecureLocal && u.Scheme != "https" {
		return errors.New("HTTPS APP_ORIGIN is required unless AUTH_INSECURE_LOCAL=true")
	}
	return nil
}

type User struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	IsOwner     bool   `json:"isOwner"`
}
type Club struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Timezone string   `json:"timezone"`
	Archived bool     `json:"archived"`
	Roles    []string `json:"roles"`
}
type principal struct {
	User                User
	tokenHash, csrfHash []byte
}
type contextKey struct{}

// CurrentUser is populated only by Require; domain handlers do not trust client user IDs.
func CurrentUser(ctx context.Context) User { return ctx.Value(contextKey{}).(principal).User }

// LockClubAdministrator repeats the live session and role checks in domain writes.
// The caller must already be inside Require and retain these locks until commit.
func LockClubAdministrator(ctx context.Context, tx pgx.Tx, clubID string) (bool, error) {
	owner, err := LockSession(ctx, tx)
	if err != nil {
		return false, err
	}
	p := ctx.Value(contextKey{}).(principal)
	var live bool
	if !owner {
		if err := tx.QueryRow(ctx, `SELECT true FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id::text=$2 AND role='administrator' FOR SHARE`, p.User.ID, clubID).Scan(&live); err != nil {
			return false, err
		}
	}
	return owner, nil
}

// LockSession rechecks the authenticated actor and session inside a domain
// transaction. Retain these locks while checking domain-specific assignments.
func LockSession(ctx context.Context, tx pgx.Tx) (bool, error) {
	p := ctx.Value(contextKey{}).(principal)
	var owner, live bool
	if err := tx.QueryRow(ctx, `SELECT is_owner FROM pride.staff_users WHERE id=$1 AND disabled_at IS NULL FOR SHARE`, p.User.ID).Scan(&owner); err != nil {
		return false, err
	}
	if err := tx.QueryRow(ctx, `SELECT true FROM pride.staff_sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now() FOR SHARE`, p.tokenHash).Scan(&live); err != nil {
		return false, err
	}
	return owner, nil
}

type Service struct {
	pool      *pgxpool.Pool
	cfg       Config
	attempts  limiter
	hashing   chan struct{}
	dummyHash string
}

func New(pool *pgxpool.Pool, cfg Config) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	dummy, err := HashPassword(randomToken())
	if err != nil {
		return nil, errors.New("cannot initialize password verification")
	}
	return &Service{pool: pool, cfg: cfg, attempts: limiter{entries: map[string]attempts{}}, hashing: make(chan struct{}, 2), dummyHash: dummy}, nil
}
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read terminates if secure randomness fails.
	return base64.RawURLEncoding.EncodeToString(b)
}
func digest(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }
func validToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32
}
func jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func (s *Service) cookieNames() (string, string) {
	if s.cfg.InsecureLocal {
		return "pride_session_local", "pride_csrf_local"
	}
	return "__Host-pride_session", "__Host-pride_csrf"
}
func (s *Service) cookies(w http.ResponseWriter, token, csrf string, expires time.Time, maxAge int) {
	sessionName, csrfName := s.cookieNames()
	for name, value := range map[string]string{sessionName: token, csrfName: csrf} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: !s.cfg.InsecureLocal, HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge})
	}
}
func (s *Service) clearCookies(w http.ResponseWriter) { s.cookies(w, "", "", time.Unix(1, 0), -1) }
func (s *Service) sameOrigin(r *http.Request) bool    { return r.Header.Get("Origin") == s.cfg.Origin }

// A stale unauthorized response must not clear a newer session cookie.
// Only explicit logout deletes cookies; login overwrites invalid/expired ones.
func unauthorized(w http.ResponseWriter) {
	platform.WriteError(w, 401, "unauthorized", "Войдите в систему.")
}
func unavailable(w http.ResponseWriter) {
	platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
}

// Require checks the live account and session on every request, plus CSRF for mutations.
func (s *Service) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionName, _ := s.cookieNames()
		cookie, err := r.Cookie(sessionName)
		if err != nil || !validToken(cookie.Value) {
			unauthorized(w)
			return
		}
		p := principal{tokenHash: digest(cookie.Value)}
		err = s.pool.QueryRow(r.Context(), `SELECT u.id::text,u.login,u.display_name,u.is_owner,ss.csrf_token_hash
            FROM pride.staff_sessions ss JOIN pride.staff_users u ON u.id=ss.staff_user_id
            WHERE ss.token_hash=$1 AND ss.revoked_at IS NULL AND ss.expires_at>now() AND u.disabled_at IS NULL`, p.tokenHash).Scan(&p.User.ID, &p.User.Login, &p.User.DisplayName, &p.User.IsOwner, &p.csrfHash)
		if errors.Is(err, pgx.ErrNoRows) {
			unauthorized(w)
			return
		}
		if err != nil {
			unavailable(w)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			csrf := r.Header.Get("X-CSRF-Token")
			if !s.sameOrigin(r) || !validToken(csrf) || subtle.ConstantTimeCompare(digest(csrf), p.csrfHash) != 1 {
				platform.WriteError(w, 403, "csrf_failed", "Обновите страницу и повторите действие.")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, p)))
	})
}

// RequireClub re-reads current assignments and account state, including for domain transactions.
// roles must be supplied by the handler (administrator/trainer); owner always has access.
// Trainer membership permits club discovery only; group/session assignment is checked by training.
func (s *Service) RequireClub(ctx context.Context, userID, clubID string, roles ...string) (Club, error) {
	var c Club
	err := s.pool.QueryRow(ctx, `SELECT c.id::text,c.name,c.timezone,c.archived_at IS NOT NULL,
        CASE WHEN u.is_owner THEN ARRAY['owner']::text[] ELSE
            ARRAY(SELECT r.role FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.club_id=c.id ORDER BY r.role) END
        FROM pride.clubs c JOIN pride.staff_users u ON u.id=$1
        WHERE c.id::text=$2 AND u.disabled_at IS NULL AND (u.is_owner OR EXISTS
            (SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.club_id=c.id AND r.role=ANY($3::text[])))`, userID, clubID, roles).Scan(&c.ID, &c.Name, &c.Timezone, &c.Archived, &c.Roles)
	return c, err
}
func (s *Service) clubs(ctx context.Context, userID string) ([]Club, error) {
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,c.name,c.timezone,false,
        CASE WHEN u.is_owner THEN ARRAY['owner']::text[] ELSE
            ARRAY(SELECT r.role FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.club_id=c.id ORDER BY r.role) END
        FROM pride.clubs c JOIN pride.staff_users u ON u.id=$1
        WHERE c.archived_at IS NULL AND u.disabled_at IS NULL AND (u.is_owner OR EXISTS
            (SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.club_id=c.id)) ORDER BY c.name,c.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Club{}
	for rows.Next() {
		var c Club
		if err := rows.Scan(&c.ID, &c.Name, &c.Timezone, &c.Archived, &c.Roles); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(contextKey{}).(principal)
	_, csrfName := s.cookieNames()
	cookie, err := r.Cookie(csrfName)
	if err != nil || !validToken(cookie.Value) || subtle.ConstantTimeCompare(digest(cookie.Value), p.csrfHash) != 1 {
		unauthorized(w)
		return
	}
	clubs, err := s.clubs(r.Context(), p.User.ID)
	if err != nil {
		unavailable(w)
		return
	}
	jsonResponse(w, 200, map[string]any{"user": p.User, "clubs": clubs, "csrfToken": cookie.Value})
}
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	// Origin guards login CSRF, before any credentials are processed.
	if !s.sameOrigin(r) {
		platform.WriteError(w, 403, "csrf_failed", "Вход разрешён только со страницы приложения.")
		return
	}
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		platform.WriteError(w, 400, "invalid_input", "Проверьте логин и пароль.")
		return
	}
	login, err := NormalizeLogin(input.Login)
	if err != nil || len(input.Password) > 128 || input.Password == "" {
		platform.WriteError(w, 401, "invalid_credentials", "Неверный логин или пароль.")
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !s.attempts.allow(ip, login, time.Now()) {
		w.Header().Set("Retry-After", "900")
		platform.WriteError(w, 429, "too_many_attempts", "Слишком много попыток. Попробуйте через 15 минут.")
		return
	}
	select {
	case s.hashing <- struct{}{}:
		defer func() { <-s.hashing }()
	default:
		w.Header().Set("Retry-After", "1")
		platform.WriteError(w, 429, "too_many_attempts", "Попробуйте войти чуть позже.")
		return
	}
	var userID, hash string
	var enabled bool
	err = s.pool.QueryRow(r.Context(), "SELECT id::text,password_hash,disabled_at IS NULL FROM pride.staff_users WHERE login=$1", login).Scan(&userID, &hash, &enabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		unavailable(w)
		return
	}
	if hash == "" {
		hash = s.dummyHash
	}
	if !VerifyPassword(hash, input.Password) || userID == "" || !enabled {
		platform.WriteError(w, 401, "invalid_credentials", "Неверный логин или пароль.")
		return
	}
	token, csrf := randomToken(), randomToken()
	expires := time.Now().Add(sessionLifetime)
	// Serialize account disable/password changes with creation of a session.
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		unavailable(w)
		return
	}
	defer tx.Rollback(context.Background())
	var currentHash string
	err = tx.QueryRow(r.Context(), "SELECT password_hash FROM pride.staff_users WHERE id=$1 AND disabled_at IS NULL FOR UPDATE", userID).Scan(&currentHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && currentHash != hash) {
		platform.WriteError(w, 401, "invalid_credentials", "Неверный логин или пароль.")
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	if _, err = tx.Exec(r.Context(), "DELETE FROM pride.staff_sessions WHERE expires_at<=now()"); err != nil {
		unavailable(w)
		return
	}
	if _, err = tx.Exec(r.Context(), "INSERT INTO pride.staff_sessions(staff_user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,$4)", userID, digest(token), digest(csrf), expires); err != nil {
		unavailable(w)
		return
	}
	// Rotate a previous session on account switch instead of leaving it usable.
	sessionName, _ := s.cookieNames()
	if old, err := r.Cookie(sessionName); err == nil && validToken(old.Value) {
		if _, err = tx.Exec(r.Context(), "UPDATE pride.staff_sessions SET revoked_at=now() WHERE token_hash=$1", digest(old.Value)); err != nil {
			unavailable(w)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w)
		return
	}
	s.cookies(w, token, csrf, expires, int(sessionLifetime.Seconds()))
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(contextKey{}).(principal)
	if _, err := s.pool.Exec(r.Context(), "UPDATE pride.staff_sessions SET revoked_at=now() WHERE token_hash=$1", p.tokenHash); err != nil {
		unavailable(w)
		return
	}
	s.clearCookies(w)
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}
func (s *Service) club(w http.ResponseWriter, r *http.Request) {
	c, err := s.RequireClub(r.Context(), CurrentUser(r.Context()).ID, r.PathValue("clubID"), "administrator", "trainer")
	if errors.Is(err, pgx.ErrNoRows) {
		platform.WriteError(w, 404, "not_found", "Филиал не найден.")
		return
	}
	if err != nil {
		unavailable(w)
		return
	}
	jsonResponse(w, 200, c)
}
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/clubs/{clubID}/trainers", s.trainerScope(s.listTrainers, false))
	mux.Handle("POST /api/clubs/{clubID}/trainers", s.trainerScope(s.createTrainer, false))
	mux.Handle("PUT /api/clubs/{clubID}/trainers/{staffID}/account", s.trainerScope(s.changeTrainer, false))
	mux.Handle("POST /api/clubs/{clubID}/trainers/{staffID}/password", s.trainerScope(s.trainerPassword, false))
	mux.Handle("POST /api/clubs/{clubID}/trainers/{staffID}/revoke", s.trainerScope(s.revokeTrainer, false))
	mux.Handle("GET /api/trainer-accounts", s.trainerScope(s.trainerRoster, true))
	mux.Handle("PUT /api/staff/{staffID}/account", s.trainerScope(s.changeTrainer, true))
	mux.Handle("POST /api/staff/{staffID}/password", s.trainerScope(s.trainerPassword, true))
	mux.Handle("POST /api/staff/{staffID}/trainer-assignments", s.trainerScope(s.assignTrainer, true))
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.Handle("GET /api/auth/me", s.Require(http.HandlerFunc(s.me)))
	mux.Handle("POST /api/auth/logout", s.Require(http.HandlerFunc(s.logout)))
	mux.Handle("GET /api/auth/clubs/{clubID}", s.Require(http.HandlerFunc(s.club)))
	mux.HandleFunc("/api/auth/", func(w http.ResponseWriter, r *http.Request) {
		platform.WriteError(w, 404, "not_found", "Адрес не найден.")
	})
	mux.Handle("/", fallback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		// Keep all auth responses (including method/path errors) out of caches.
		if strings.HasPrefix(r.URL.Path, "/api/auth/") || strings.HasPrefix(r.URL.Path, "/api/clubs/") || strings.HasPrefix(r.URL.Path, "/api/staff/") || r.URL.Path == "/api/trainer-accounts" {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}
