// Package attendance freezes a session roster and serializes journal commands.
package attendance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errInput = errors.New("invalid attendance input")
var errState = errors.New("journal state conflict")
var errVersion = errors.New("journal version conflict")
var errFuture = errors.New("session date has not arrived")

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
	now  func() time.Time
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access, time.Now} }

type Record struct {
	ID           string     `json:"id"`
	MembershipID string     `json:"membershipId"`
	SourceID     string     `json:"sourceGroupMembershipId"`
	Name         string     `json:"fullName"`
	Archived     bool       `json:"archived"`
	State        *string    `json:"state"`
	Reason       *string    `json:"absenceReason"`
	MarkedBy     *string    `json:"markedBy"`
	MarkedAt     *time.Time `json:"markedAt"`
	Origin       *string    `json:"origin"`
	Version      int        `json:"version"`
}
type Journal struct {
	Session sessions.Session `json:"session"`
	Records []Record         `json:"records"`
}

func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	var p *pgconn.PgError
	if errors.As(e, &p) {
		switch p.Code {
		case "22P02", "23503":
			e = pgx.ErrNoRows
		case "23514":
			e = errState
		}
	}
	switch {
	case errors.Is(e, errInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте дату, версию и отметку посещаемости.")
	case errors.Is(e, errVersion):
		platform.WriteError(w, 409, "version_conflict", "Журнал изменился. Обновите его и проверьте отметки перед повтором.")
	case errors.Is(e, errState):
		platform.WriteError(w, 409, "journal_state_conflict", "Журнал недоступен для записи. Обновите занятие.")
	case errors.Is(e, errFuture):
		platform.WriteError(w, 409, "session_not_available", "Журнал можно открыть с локальной даты занятия.")
	case errors.Is(e, clubs.ErrArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Запись журнала недоступна.")
	case errors.Is(e, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Занятие или отметка недоступны.")
	default:
		platform.WriteError(w, 503, "not_ready", "Журнал временно недоступен. Попробуйте снова.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, in any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(in) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, errInput)
		return false
	}
	return true
}
func (s *Service) begin(r *http.Request, write bool) (pgx.Tx, error) {
	ctx := r.Context()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	_, e = auth.LockSession(ctx, tx)
	if e == nil {
		e = auth.RequireTrainer(ctx, tx, auth.CurrentUser(ctx).ID, r.PathValue("clubID"))
	}
	if e == nil {
		e = schedule.Lock(ctx, tx)
	}
	if e == nil && write {
		e = clubs.RequireActive(ctx, tx, r.PathValue("clubID"))
	}
	if e != nil {
		_ = tx.Rollback(context.Background())
		return nil, e
	}
	return tx, nil
}
func (s *Service) session(r *http.Request, tx pgx.Tx) (sessions.Session, error) {
	v, e := sessions.Read(r.Context(), tx, r.PathValue("clubID"), r.PathValue("sessionID"), true)
	if e == nil && (v.TrainerID != auth.CurrentUser(r.Context()).ID || v.SupersededAt != nil) {
		e = pgx.ErrNoRows
	}
	return v, e
}
func records(ctx context.Context, tx pgx.Tx, id string) ([]Record, error) {
	rows, e := tx.Query(ctx, `SELECT a.id::text,a.club_membership_id::text,a.source_group_membership_id::text,p.full_name,c.archived_at IS NOT NULL,a.state,a.absence_reason,a.marked_by::text,a.marked_at,a.origin,a.version FROM pride.attendance_records a JOIN pride.club_memberships c ON c.id=a.club_membership_id JOIN pride.participants p ON p.id=c.participant_id WHERE a.training_session_id::text=$1 ORDER BY p.full_name,a.id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var v Record
		if e = rows.Scan(&v.ID, &v.MembershipID, &v.SourceID, &v.Name, &v.Archived, &v.State, &v.Reason, &v.MarkedBy, &v.MarkedAt, &v.Origin, &v.Version); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func journal(ctx context.Context, tx pgx.Tx, v sessions.Session) (Journal, error) {
	rs, e := records(ctx, tx, v.ID)
	return Journal{v, rs}, e
}
func recordState(v Record) map[string]any {
	return map[string]any{"sessionId": nil, "state": v.State, "absenceReason": v.Reason, "markedBy": v.MarkedBy, "markedAt": v.MarkedAt, "origin": v.Origin, "version": v.Version}
}
func auditRecord(ctx context.Context, tx pgx.Tx, actor string, v sessions.Session, old, new Record) error {
	before := recordState(old)
	after := recordState(new)
	before["sessionId"] = v.ID
	after["sessionId"] = v.ID
	return groups.Audit(ctx, tx, actor, v.ClubID, new.ID, "attendance_record", "marked", before, after)
}
func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	tx, e := s.begin(r, false)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	v, e := s.session(r, tx)
	var out Journal
	if e == nil {
		out, e = journal(r.Context(), tx, v)
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, out)
}

type versionInput struct {
	Version int `json:"version"`
}

func (s *Service) open(w http.ResponseWriter, r *http.Request) {
	var in versionInput
	if !decode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		fail(w, errInput)
		return
	}
	tx, e := s.begin(r, true)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	ctx := r.Context()
	v, e := s.session(r, tx)
	if e == nil && v.Status == "cancelled" {
		e = errState
	}
	if e == nil && v.Status == "planned" {
		if v.Version != in.Version {
			e = errVersion
		} else {
			loc, err := time.LoadLocation(v.Timezone)
			if err != nil {
				e = err
			} else if v.Date > s.now().In(loc).Format("2006-01-02") {
				e = errFuture
			}
		}
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) SELECT $1,$2,$3,m.club_membership_id,m.id FROM pride.group_memberships m WHERE m.club_id=$1 AND m.group_id=$3 AND m.valid_from<=$4::date AND (m.valid_until IS NULL OR $4::date<m.valid_until)`, v.ClubID, v.ID, v.GroupID, v.Date)
		}
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE pride.training_sessions SET status='open',roster_frozen_at=now(),version=version+1,updated_at=now() WHERE id=$1`, v.ID)
		}
		if e == nil {
			e = groups.Audit(ctx, tx, auth.CurrentUser(ctx).ID, v.ClubID, v.ID, "training_session", "journal_opened", map[string]any{"status": "planned", "version": v.Version}, map[string]any{"status": "open", "version": v.Version + 1})
		}
		if e == nil {
			v, e = s.session(r, tx)
		}
	}
	var out Journal
	if e == nil {
		out, e = journal(ctx, tx, v)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, out)
}

type markInput struct {
	Version        int     `json:"version"`
	SessionVersion int     `json:"sessionVersion"`
	State          string  `json:"state"`
	Reason         *string `json:"absenceReason"`
}

func (s *Service) mark(w http.ResponseWriter, r *http.Request) {
	var in markInput
	if !decode(w, r, &in) {
		return
	}
	valid := in.Version > 0 && in.SessionVersion > 0 && ((in.State == "present" && in.Reason == nil) || (in.State == "absent" && in.Reason != nil && (*in.Reason == "unknown" || *in.Reason == "illness" || *in.Reason == "excused")))
	if !valid {
		fail(w, errInput)
		return
	}
	tx, e := s.begin(r, true)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	ctx := r.Context()
	v, e := s.session(r, tx)
	if e == nil && v.Status != "open" {
		e = errState
	}
	var old Record
	if e == nil {
		rs, err := records(ctx, tx, v.ID)
		e = err
		found := false
		for _, a := range rs {
			if a.ID == r.PathValue("recordID") {
				old = a
				found = true
				break
			}
		}
		if e == nil && !found {
			e = pgx.ErrNoRows
		}
	}
	if e == nil {
		identical := old.State != nil && *old.State == in.State && ((old.Reason == nil && in.Reason == nil) || (old.Reason != nil && in.Reason != nil && *old.Reason == *in.Reason))
		// Same effective value is an idempotent retry, even after another command.
		if !identical {
			if old.Version != in.Version || v.Version != in.SessionVersion {
				e = errVersion
			} else {
				_, e = tx.Exec(ctx, `UPDATE pride.attendance_records SET state=$2,absence_reason=$3,marked_by=$4,marked_at=now(),origin='manual',version=version+1,updated_at=now() WHERE id=$1`, old.ID, in.State, in.Reason, auth.CurrentUser(ctx).ID)
				if e == nil {
					rs, err := records(ctx, tx, v.ID)
					e = err
					for _, a := range rs {
						if a.ID == old.ID && e == nil {
							e = auditRecord(ctx, tx, auth.CurrentUser(ctx).ID, v, old, a)
						}
					}
				}
				if e == nil {
					_, e = tx.Exec(ctx, `UPDATE pride.training_sessions SET version=version+1,updated_at=now() WHERE id=$1`, v.ID)
				}
			}
		}
	}
	var out Journal
	if e == nil {
		v, e = s.session(r, tx)
	}
	if e == nil {
		out, e = journal(ctx, tx, v)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, out)
}
func (s *Service) close(w http.ResponseWriter, r *http.Request) {
	var in versionInput
	if !decode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		fail(w, errInput)
		return
	}
	tx, e := s.begin(r, true)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	ctx := r.Context()
	v, e := s.session(r, tx)
	if e == nil && v.Status != "closed" {
		if v.Status != "open" {
			e = errState
		} else if v.Version != in.Version {
			e = errVersion
		} else {
			rs, err := records(ctx, tx, v.ID)
			e = err
			if e == nil {
				_, e = tx.Exec(ctx, `UPDATE pride.attendance_records SET state='absent',absence_reason='unknown',marked_by=$2,marked_at=now(),origin='close_default',version=version+1,updated_at=now() WHERE training_session_id=$1 AND state IS NULL`, v.ID, auth.CurrentUser(ctx).ID)
			}
			if e == nil {
				updated, err := records(ctx, tx, v.ID)
				e = err
				byID := map[string]Record{}
				for _, a := range updated {
					byID[a.ID] = a
				}
				for _, old := range rs {
					if old.State == nil && e == nil {
						e = auditRecord(ctx, tx, auth.CurrentUser(ctx).ID, v, old, byID[old.ID])
					}
				}
			}
			if e == nil {
				_, e = tx.Exec(ctx, `UPDATE pride.training_sessions SET status='closed',closed_at=now(),version=version+1,updated_at=now() WHERE id=$1`, v.ID)
			}
			if e == nil {
				e = groups.Audit(ctx, tx, auth.CurrentUser(ctx).ID, v.ClubID, v.ID, "training_session", "journal_closed", map[string]any{"status": "open", "version": v.Version}, map[string]any{"status": "closed", "version": v.Version + 1})
			}
		}
	}
	var out Journal
	if e == nil {
		v, e = s.session(r, tx)
	}
	if e == nil {
		out, e = journal(ctx, tx, v)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, out)
}
func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	tx, e := s.begin(r, false)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	ctx := r.Context()
	date := r.URL.Query().Get("date")
	if date == "" {
		var zone string
		e = tx.QueryRow(ctx, `SELECT timezone FROM pride.clubs WHERE id::text=$1`, r.PathValue("clubID")).Scan(&zone)
		if e == nil {
			var loc *time.Location
			loc, e = time.LoadLocation(zone)
			if e == nil {
				date = s.now().In(loc).Format("2006-01-02")
			}
		}
	}
	if e == nil && !groups.ValidDate(date) {
		e = errInput
	}
	ids := []string{}
	if e == nil {
		var rows pgx.Rows
		rows, e = tx.Query(ctx, `SELECT id::text FROM pride.training_sessions WHERE club_id::text=$1 AND trainer_id=$2 AND superseded_at IS NULL AND (starts_at AT TIME ZONE timezone)::date=$3::date ORDER BY starts_at,id`, r.PathValue("clubID"), auth.CurrentUser(ctx).ID, date)
		if e == nil {
			for rows.Next() {
				var id string
				if e = rows.Scan(&id); e != nil {
					break
				}
				ids = append(ids, id)
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
		}
	}
	out := []sessions.Session{}
	if e == nil {
		for _, id := range ids {
			v, err := sessions.Read(ctx, tx, r.PathValue("clubID"), id, false)
			if err != nil {
				e = err
				break
			}
			out = append(out, v)
		}
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, out)
}
func (s *Service) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	base := "/api/clubs/{clubID}/trainer-sessions"
	mux.Handle("GET "+base, s.auth.Require(http.HandlerFunc(s.list)))
	journal := base + "/{sessionID}/journal"
	mux.Handle("GET "+journal, s.auth.Require(http.HandlerFunc(s.get)))
	mux.Handle("POST "+journal+"/open", s.auth.Require(http.HandlerFunc(s.open)))
	mux.Handle("POST "+journal+"/close", s.auth.Require(http.HandlerFunc(s.close)))
	mux.Handle("PUT "+journal+"/records/{recordID}", s.auth.Require(http.HandlerFunc(s.mark)))
	mux.Handle("/", next)
	return mux
}
