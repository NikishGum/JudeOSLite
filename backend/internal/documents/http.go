// Package documents stores private membership files. Content is never exposed
// through static hosting, JSON metadata, audit payloads or a public URL.
package documents

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/families"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }

type MemberDocument struct {
	ID           string    `json:"id"`
	MembershipID string    `json:"membershipId"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	Filename     string    `json:"filename"`
	MediaType    string    `json:"mediaType"`
	Size         int       `json:"size"`
	ExpiresOn    *string   `json:"expiresOn"`
	CreatedAt    time.Time `json:"createdAt"`
	Version      int       `json:"version"`
}

type documentList struct {
	Items   []MemberDocument `json:"items"`
	Total   int              `json:"total"`
	HasMore bool             `json:"hasMore"`
}

const columns = `id::text,membership_id::text,name,kind,filename,media_type,size,expires_on::text,created_at,version`

func scan(row pgx.Row) (MemberDocument, error) {
	var d MemberDocument
	err := row.Scan(&d.ID, &d.MembershipID, &d.Name, &d.Kind, &d.Filename, &d.MediaType, &d.Size, &d.ExpiresOn, &d.CreatedAt, &d.Version)
	return d, err
}
func validID(raw string) bool { var id pgtype.UUID; return id.Scan(raw) == nil && id.Valid }

func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	base := "/api/clubs/{clubID}/participants/{membershipID}/documents"
	mux.Handle("GET "+base, s.auth.Require(http.HandlerFunc(s.list)))
	mux.Handle("POST "+base, s.auth.Require(http.HandlerFunc(s.upload)))
	mux.Handle("GET "+base+"/{documentID}/download", s.auth.Require(http.HandlerFunc(s.download)))
	mux.Handle("POST "+base+"/{documentID}/delete", s.auth.Require(http.HandlerFunc(s.delete)))
	mux.Handle("/", fallback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/documents") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
		}
		mux.ServeHTTP(w, r)
	})
}
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, err error) {
	var p *pgconn.PgError
	switch {
	case errors.Is(err, errInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте название, тип, имя файла, дату и содержимое PDF, PNG или JPEG.")
	case errors.Is(err, errSize):
		platform.WriteError(w, 413, "file_too_large", "Файл должен быть не больше 5 МиБ, JSON — не больше 7 МиБ.")
	case errors.Is(err, errQuota):
		platform.WriteError(w, 409, "version_conflict", "Лимит карточки: 20 документов и 25 МиБ. Удалите ненужные файлы.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Карточка или документ недоступны.")
	case errors.Is(err, clubs.ErrArchived):
		platform.WriteError(w, 409, "version_conflict", "Филиал в архиве. Изменения запрещены.")
	case errors.Is(err, errVersion), errors.As(err, &p) && (p.Code == "40001" || p.Code == "40P01"):
		platform.WriteError(w, 409, "version_conflict", "Документ изменён. Обновите список.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || ct != "application/json" {
		fail(w, errInput)
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	d.DisallowUnknownFields()
	err = d.Decode(dst)
	if err == nil {
		err = d.Decode(&struct{}{})
		if err == io.EOF {
			return true
		}
	}
	var max *http.MaxBytesError
	if errors.As(err, &max) {
		fail(w, errSize)
	} else {
		fail(w, errInput)
	}
	return false
}

// All operations repeat live authorization within the transaction. Writers lock
// the membership before counting quota; competing uploads cannot exceed it.
func (s *Service) transaction(r *http.Request, write bool, fn func(pgx.Tx) error) error {
	club, member := r.PathValue("clubID"), r.PathValue("membershipID")
	if !validID(club) || !validID(member) || r.PathValue("documentID") != "" && !validID(r.PathValue("documentID")) {
		return pgx.ErrNoRows
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if write {
		_, err = auth.LockClubAdministrator(r.Context(), tx, club)
		if err == nil {
			err = clubs.RequireActive(r.Context(), tx, club)
		}
		if err == nil {
			var live bool
			err = tx.QueryRow(r.Context(), `SELECT true FROM pride.club_memberships WHERE club_id=$1 AND id=$2 FOR UPDATE`, club, member).Scan(&live)
		}
	} else {
		err = families.LockMemberReader(r.Context(), tx, club, member)
	}
	if err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}
func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	page, limit := 0, 25
	var err error
	for key, target := range map[string]*int{"page": &page, "limit": &limit} {
		if values, ok := r.URL.Query()[key]; ok {
			if len(values) != 1 {
				fail(w, errInput)
				return
			}
			*target, err = strconv.Atoi(values[0])
			if err != nil {
				fail(w, errInput)
				return
			}
		}
	}
	q := r.URL.Query().Get("q")
	if page < 0 || page > 1000000 || limit < 1 || limit > 100 || !safeText(q, 200) {
		fail(w, errInput)
		return
	}
	result := documentList{Items: []MemberDocument{}}
	err = s.transaction(r, false, func(tx pgx.Tx) error {
		ctx := r.Context()
		member := r.PathValue("membershipID")
		filter := ` FROM pride.member_documents WHERE membership_id=$1 AND club_id=$2 AND ($3='' OR strpos(lower(name),lower($3))>0)`
		if err := tx.QueryRow(ctx, `SELECT count(*)`+filter, member, r.PathValue("clubID"), q).Scan(&result.Total); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT `+columns+filter+` ORDER BY created_at DESC,id DESC LIMIT $4 OFFSET $5`, member, r.PathValue("clubID"), q, limit, page*limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scan(rows)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, d)
		}
		return rows.Err()
	})
	if err != nil {
		fail(w, err)
		return
	}
	result.HasMore = (page*limit + len(result.Items)) < result.Total
	respond(w, 200, result)
}
func (s *Service) upload(w http.ResponseWriter, r *http.Request) {
	var in uploadInput
	if !decode(w, r, &in, MaxBodyBytes) {
		return
	}
	data, err := validate(in)
	if err != nil {
		fail(w, err)
		return
	}
	var result MemberDocument
	err = s.transaction(r, true, func(tx pgx.Tx) error {
		ctx := r.Context()
		club, member := r.PathValue("clubID"), r.PathValue("membershipID")
		var count, size int
		if err := tx.QueryRow(ctx, `SELECT count(*),coalesce(sum(size),0) FROM pride.member_documents WHERE membership_id=$1`, member).Scan(&count, &size); err != nil {
			return err
		}
		if count >= MaxDocuments || size+len(data.content) > MaxMemberBytes {
			return errQuota
		}
		var err error
		result, err = scan(tx.QueryRow(ctx, `INSERT INTO pride.member_documents(club_id,membership_id,name,kind,filename,media_type,content,expires_on,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+columns, club, member, data.Name, data.Kind, data.Filename, data.mediaType, data.content, data.expiry, auth.CurrentUser(ctx).ID))
		if err != nil {
			return err
		}
		return audit(ctx, tx, club, auth.CurrentUser(ctx).ID, result, "uploaded")
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, result)
}
func (s *Service) download(w http.ResponseWriter, r *http.Request) {
	var result MemberDocument
	var content []byte
	err := s.transaction(r, false, func(tx pgx.Tx) error {
		var err error
		result, err = scan(tx.QueryRow(r.Context(), `SELECT `+columns+` FROM pride.member_documents WHERE id=$1 AND membership_id=$2 AND club_id=$3`, r.PathValue("documentID"), r.PathValue("membershipID"), r.PathValue("clubID")))
		if err != nil {
			return err
		}
		return tx.QueryRow(r.Context(), `SELECT content FROM pride.member_documents WHERE id=$1`, result.ID).Scan(&content)
	})
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", result.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": result.Filename}))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(200)
	if r.Method != "HEAD" {
		_, _ = w.Write(content)
	}
}
func (s *Service) delete(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version *int `json:"version"`
	}
	if !decode(w, r, &in, 1024) {
		return
	}
	if in.Version == nil || *in.Version < 1 {
		fail(w, errInput)
		return
	}
	err := s.transaction(r, true, func(tx pgx.Tx) error {
		d, err := scan(tx.QueryRow(r.Context(), `SELECT `+columns+` FROM pride.member_documents WHERE id=$1 AND membership_id=$2 AND club_id=$3 FOR UPDATE`, r.PathValue("documentID"), r.PathValue("membershipID"), r.PathValue("clubID")))
		if err != nil {
			return err
		}
		if d.Version != *in.Version {
			return errVersion
		}
		if _, err = tx.Exec(r.Context(), `DELETE FROM pride.member_documents WHERE id=$1`, d.ID); err != nil {
			return err
		}
		return audit(r.Context(), tx, r.PathValue("clubID"), auth.CurrentUser(r.Context()).ID, d, "deleted")
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, map[string]string{"status": "ok"})
}
func audit(ctx context.Context, tx pgx.Tx, club, actor string, d MemberDocument, action string) error {
	// No user text, filename, expiry, contents or medical information in history.
	state, _ := json.Marshal(map[string]any{"membershipId": d.MembershipID, "version": d.Version, "mediaType": d.MediaType, "size": d.Size})
	_, err := tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'member_document',$3,$4,$5)`, club, actor, d.ID, "member_document_"+action, state)
	return err
}
