// Package corrections provides club-scoped journal reading and single closed-mark corrections.
package corrections

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errInput = errors.New("invalid correction")
var errVersion = errors.New("record version conflict")
var errState = errors.New("journal is not closed")

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }

type Record struct {
	attendance.Record
	ChangeReason *string `json:"changeReason"`
}
type Journal struct {
	Session sessions.Session  `json:"session"`
	Records []Record          `json:"records"`
	Roster  attendance.Roster `json:"roster"`
}

// Whitelisted audit fields only; never return arbitrary before/after JSON.
type Value struct {
	SessionID    string     `json:"sessionId"`
	State        *string    `json:"state"`
	Reason       *string    `json:"absenceReason"`
	MarkedBy     *string    `json:"markedBy"`
	MarkedAt     *time.Time `json:"markedAt"`
	Origin       *string    `json:"origin"`
	Version      int        `json:"version"`
	ChangeReason *string    `json:"changeReason"`
}
type Event struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actorId"`
	ActorName    string    `json:"actorName"`
	CreatedAt    time.Time `json:"createdAt"`
	Action       string    `json:"action"`
	Before       Value     `json:"before"`
	After        Value     `json:"after"`
	ChangeReason *string   `json:"changeReason"`
}

func reply(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
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
		platform.WriteError(w, 400, "invalid_input", "Укажите версию, отметку и причину исправления (1–1000 символов).")
	case errors.Is(e, errVersion):
		platform.WriteError(w, 409, "version_conflict", "Отметка уже изменена. Перечитайте журнал и проверьте актуальное значение.")
	case errors.Is(e, errState):
		platform.WriteError(w, 409, "journal_state_conflict", "Исправление доступно только в закрытом журнале.")
	case errors.Is(e, clubs.ErrArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Исправление недоступно.")
	case errors.Is(e, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Журнал или отметка недоступны.")
	default:
		platform.WriteError(w, 503, "not_ready", "Журнал временно недоступен. Повторите запрос.")
	}
}
func (s *Service) begin(r *http.Request, write bool) (pgx.Tx, error) {
	ctx := r.Context()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	_, e = auth.LockClubAdministrator(ctx, tx, r.PathValue("clubID"))
	if e == nil {
		e = schedule.Lock(ctx, tx)
	}
	if e == nil && write {
		e = clubs.RequireActive(ctx, tx, r.PathValue("clubID"))
	}
	if e == nil && !write {
		var exists bool
		e = tx.QueryRow(ctx, `SELECT true FROM pride.clubs WHERE id::text=$1`, r.PathValue("clubID")).Scan(&exists)
	}
	if e != nil {
		_ = tx.Rollback(context.Background())
		return nil, e
	}
	return tx, nil
}
func readSession(r *http.Request, tx pgx.Tx) (sessions.Session, error) {
	v, e := sessions.Read(r.Context(), tx, r.PathValue("clubID"), r.PathValue("sessionID"), true)
	if e == nil && v.SupersededAt != nil {
		e = pgx.ErrNoRows
	}
	return v, e
}
func readJournal(ctx context.Context, tx pgx.Tx, v sessions.Session) (Journal, error) {
	out := Journal{Session: v, Records: []Record{}}
	records, e := attendance.ReadRecords(ctx, tx, v.ID)
	if e != nil {
		return out, e
	}
	out.Roster, e = attendance.ReadRoster(ctx, tx, v, records)
	if e != nil {
		return out, e
	}
	reasons := map[string]*string{}
	rows, e := tx.Query(ctx, `SELECT id::text,change_reason FROM pride.attendance_records WHERE training_session_id=$1`, v.ID)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var reason *string
		if e = rows.Scan(&id, &reason); e != nil {
			return out, e
		}
		reasons[id] = reason
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	for _, a := range records {
		out.Records = append(out.Records, Record{a, reasons[a.ID]})
	}
	return out, nil
}
func findRecord(j Journal, id string) (Record, error) {
	for _, a := range j.Records {
		if a.ID == id {
			return a, nil
		}
	}
	return Record{}, pgx.ErrNoRows
}
func value(v Record, sessionID string) Value {
	return Value{sessionID, v.State, v.Reason, v.MarkedBy, v.MarkedAt, v.Origin, v.Version, v.ChangeReason}
}
func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	tx, e := s.begin(r, false)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	v, e := readSession(r, tx)
	var out Journal
	if e == nil {
		out, e = readJournal(r.Context(), tx, v)
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, e)
		return
	}
	reply(w, out)
}

type Input struct {
	Version      int     `json:"version"`
	State        string  `json:"state"`
	Reason       *string `json:"absenceReason"`
	ChangeReason string  `json:"changeReason"`
}

func (s *Service) correct(w http.ResponseWriter, r *http.Request) {
	var in Input
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, errInput)
		return
	}
	in.ChangeReason = strings.TrimSpace(in.ChangeReason)
	valid := in.Version > 0 && utf8.ValidString(in.ChangeReason) && utf8.RuneCountInString(in.ChangeReason) >= 1 && utf8.RuneCountInString(in.ChangeReason) <= 1000
	for _, c := range in.ChangeReason {
		if unicode.IsControl(c) && c != '\n' && c != '\t' {
			valid = false
		}
	}
	valid = valid && ((in.State == "present" && in.Reason == nil) || (in.State == "absent" && in.Reason != nil && (*in.Reason == "unknown" || *in.Reason == "illness" || *in.Reason == "excused")))
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
	v, e := readSession(r, tx)
	if e == nil && v.Status != "closed" {
		e = errState
	}
	var out Journal
	var old Record
	if e == nil {
		out, e = readJournal(ctx, tx, v)
	}
	if e == nil {
		old, e = findRecord(out, r.PathValue("recordID"))
	}
	// Check the expected record version even for identical values: a stale edit
	// must never silently claim to have saved a different administrator's reason.
	if e == nil && old.Version != in.Version {
		e = errVersion
	}
	if e == nil {
		same := old.State != nil && *old.State == in.State && ((old.Reason == nil && in.Reason == nil) || (old.Reason != nil && in.Reason != nil && *old.Reason == *in.Reason))
		if !same {
			_, e = tx.Exec(ctx, `UPDATE pride.attendance_records SET state=$2,absence_reason=$3,marked_by=$4,marked_at=now(),origin='correction',change_reason=$5,version=version+1,updated_at=now() WHERE id=$1`, old.ID, in.State, in.Reason, auth.CurrentUser(ctx).ID, in.ChangeReason)
			if e == nil {
				out, e = readJournal(ctx, tx, v)
			}
			var updated Record
			if e == nil {
				updated, e = findRecord(out, old.ID)
			}
			if e == nil {
				before, _ := json.Marshal(value(old, v.ID))
				after, _ := json.Marshal(value(updated, v.ID))
				_, e = tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,before_state,after_state,change_reason) VALUES($1,$2,'attendance_record',$3,'corrected',$4,$5,$6)`, v.ClubID, auth.CurrentUser(ctx).ID, old.ID, before, after, in.ChangeReason)
			}
		}
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fail(w, e)
		return
	}
	reply(w, out)
}
func (s *Service) history(w http.ResponseWriter, r *http.Request) {
	tx, e := s.begin(r, false)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	ctx := r.Context()
	v, e := readSession(r, tx)
	var j Journal
	if e == nil {
		j, e = readJournal(ctx, tx, v)
	}
	if e == nil {
		_, e = findRecord(j, r.PathValue("recordID"))
	}
	out := []Event{}
	if e == nil {
		var rows pgx.Rows
		rows, e = tx.Query(ctx, `SELECT a.id::text,a.actor_id::text,u.display_name,a.created_at,a.action,a.before_state,a.after_state,a.change_reason FROM pride.audit_events a JOIN pride.staff_users u ON u.id=a.actor_id WHERE a.club_id=$1 AND a.entity_type='attendance_record' AND a.entity_id::text=$2 AND a.action IN ('marked','corrected') ORDER BY a.created_at,a.id`, v.ClubID, r.PathValue("recordID"))
		if e == nil {
			for rows.Next() {
				var a Event
				var before, after []byte
				if e = rows.Scan(&a.ID, &a.ActorID, &a.ActorName, &a.CreatedAt, &a.Action, &before, &after, &a.ChangeReason); e != nil {
					break
				}
				if e = json.Unmarshal(before, &a.Before); e != nil {
					break
				}
				if e = json.Unmarshal(after, &a.After); e != nil {
					break
				}
				out = append(out, a)
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
		}
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fail(w, e)
		return
	}
	reply(w, out)
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
	group := r.URL.Query().Get("groupId")
	if !groups.ValidDate(date) {
		fail(w, errInput)
		return
	}
	// Group filtering must not accept a foreign or nonexistent ID as a valid scope.
	if group != "" {
		_, e = groups.ReadGroup(ctx, tx, r.PathValue("clubID"), group, false)
	}
	ids := []string{}
	if e == nil {
		var rows pgx.Rows
		rows, e = tx.Query(ctx, `SELECT id::text FROM pride.training_sessions WHERE club_id::text=$1 AND superseded_at IS NULL AND (starts_at AT TIME ZONE timezone)::date=$2::date AND ($3='' OR group_id::text=$3) ORDER BY starts_at,id`, r.PathValue("clubID"), date, group)
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
			var v sessions.Session
			v, e = sessions.Read(ctx, tx, r.PathValue("clubID"), id, false)
			if e != nil {
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
	reply(w, out)
}
func (s *Service) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	base := "/api/clubs/{clubID}/journals"
	j := base + "/{sessionID}"
	mux.Handle("GET "+base, s.auth.Require(http.HandlerFunc(s.list)))
	mux.Handle("GET "+j, s.auth.Require(http.HandlerFunc(s.get)))
	mux.Handle("PUT "+j+"/records/{recordID}", s.auth.Require(http.HandlerFunc(s.correct)))
	mux.Handle("GET "+j+"/records/{recordID}/history", s.auth.Require(http.HandlerFunc(s.history)))
	mux.Handle("/", next)
	return mux
}
