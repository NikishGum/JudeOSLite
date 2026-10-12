package sessions

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
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errVersion = errors.New("session version conflict")
var errState = errors.New("session state conflict")
var errConfirm = errors.New("open journal confirmation required")

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
	now  func() time.Time
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access, time.Now} }

type Session struct {
	ID           string     `json:"id"`
	ClubID       string     `json:"clubId"`
	GroupID      string     `json:"groupId"`
	GroupName    string     `json:"groupName"`
	RuleID       *string    `json:"ruleId"`
	TrainerID    string     `json:"trainerId"`
	TrainerName  string     `json:"trainerName"`
	Date         string     `json:"date"`       // Actual local calendar date, including a manual move.
	SourceDate   string     `json:"sourceDate"` // Immutable starts_on of the original occurrence.
	Timezone     string     `json:"timezone"`
	StartsAt     time.Time  `json:"startsAt"`
	EndsAt       time.Time  `json:"endsAt"`
	Status       string     `json:"status"`
	Manual       bool       `json:"manualOverride"`
	Frozen       bool       `json:"rosterFrozen"`
	ClosedAt     *time.Time `json:"closedAt"`
	CancelledAt  *time.Time `json:"cancelledAt"`
	Reason       *string    `json:"cancellationReason"`
	SupersededAt *time.Time `json:"supersededAt"`
	Version      int        `json:"version"`
}

const columns = `s.id::text,s.club_id::text,s.group_id::text,g.name,s.schedule_rule_id::text,s.trainer_id::text,u.display_name,to_char(s.starts_at AT TIME ZONE s.timezone,'YYYY-MM-DD'),to_char(s.starts_on,'YYYY-MM-DD'),s.timezone,s.starts_at,s.ends_at,s.status,s.is_manual_override,s.roster_frozen_at IS NOT NULL,s.closed_at,s.cancelled_at,s.cancellation_reason,s.superseded_at,s.version`
const joins = ` FROM pride.training_sessions s JOIN pride.training_groups g ON g.id=s.group_id JOIN pride.staff_users u ON u.id=s.trainer_id `

func scan(row pgx.Row) (Session, error) {
	var v Session
	e := row.Scan(&v.ID, &v.ClubID, &v.GroupID, &v.GroupName, &v.RuleID, &v.TrainerID, &v.TrainerName, &v.Date, &v.SourceDate, &v.Timezone, &v.StartsAt, &v.EndsAt, &v.Status, &v.Manual, &v.Frozen, &v.ClosedAt, &v.CancelledAt, &v.Reason, &v.SupersededAt, &v.Version)
	return v, e
}
func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	var p *pgconn.PgError
	if errors.As(e, &p) {
		switch p.Code {
		case "23505":
			e = schedule.ErrDuplicate
		case "23514":
			e = errState
		case "23503", "22P02":
			e = pgx.ErrNoRows
		}
	}
	switch {
	case errors.Is(e, schedule.ErrInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте дату, время, длительность, тренера и причину отмены (1–1000 символов). Разовое занятие и перенос должны быть в будущем.")
	case errors.Is(e, errVersion):
		platform.WriteError(w, 409, "version_conflict", "Занятие уже изменено. Ввод сохранён; загрузите актуальное занятие перед повторным подтверждением.")
	case errors.Is(e, errConfirm):
		platform.WriteError(w, 409, "open_confirmation_required", "Журнал открыт. Подтвердите отмену: снимок и отметки сохранятся, запись закроется, автоматических пропусков не будет.")
	case errors.Is(e, errState):
		platform.WriteError(w, 409, "session_state_conflict", "Это действие недоступно в текущем состоянии занятия. Закрытые, отменённые и заменённые планы нельзя редактировать; время открытого журнала сохраняется.")
	case errors.Is(e, schedule.ErrDuplicate):
		platform.WriteError(w, 409, "session_duplicate", "У группы уже есть занятие в это время, включая отменённое. Выберите другое время.")
	case errors.Is(e, clubs.ErrArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Изменения занятий недоступны.")
	case errors.Is(e, groups.ErrArchived):
		platform.WriteError(w, 409, "group_archived", "Группа в архиве. Новое занятие, замена тренера и перенос недоступны.")
	case errors.Is(e, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Занятие, группа, тренер или филиал недоступны.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, schedule.ErrInput)
		return false
	}
	return true
}
func (s *Service) access(write bool, fn http.HandlerFunc) http.Handler {
	return s.auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles := []string{"administrator"}
		if !write {
			roles = append(roles, "trainer")
		}
		if _, e := s.auth.RequireClub(r.Context(), auth.CurrentUser(r.Context()).ID, r.PathValue("clubID"), roles...); e != nil {
			fail(w, e)
			return
		}
		fn(w, r)
	}))
}
func (s *Service) transaction(r *http.Request, fn func(pgx.Tx) error) error {
	ctx := r.Context()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = auth.LockClubAdministrator(ctx, tx, r.PathValue("clubID")); e != nil {
		return e
	}
	// Same lock order as dated rule changes/generation; take it before session locks.
	if e = schedule.Lock(ctx, tx); e != nil {
		return e
	}
	if e = clubs.RequireActive(ctx, tx, r.PathValue("clubID")); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// No names, contacts, free-text reasons or secrets in audit payloads.
func state(v Session) any {
	return map[string]any{"groupId": v.GroupID, "ruleId": v.RuleID, "trainerId": v.TrainerID, "sourceDate": v.SourceDate, "startsAt": v.StartsAt, "endsAt": v.EndsAt, "status": v.Status, "manualOverride": v.Manual, "rosterFrozen": v.Frozen, "closedAt": v.ClosedAt, "cancelledAt": v.CancelledAt, "reasonRecorded": v.Reason != nil, "version": v.Version}
}

type createInput struct {
	GroupID   string `json:"groupId"`
	TrainerID string `json:"trainerId"`
	Date      string `json:"date"`
	Time      string `json:"startsTime"`
	Duration  int    `json:"durationMinutes"`
}

func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	var in createInput
	if !decode(w, r, &in) {
		return
	}
	var out Session
	e := s.transaction(r, func(tx pgx.Tx) error {
		ctx := r.Context()
		club := r.PathValue("clubID")
		if _, e := groups.ReadGroup(ctx, tx, club, in.GroupID, true); e != nil {
			return e
		}
		if e := auth.RequireTrainer(ctx, tx, in.TrainerID, club); e != nil {
			return e
		}
		var zone, id string
		if e := tx.QueryRow(ctx, `SELECT timezone FROM pride.clubs WHERE id::text=$1`, club).Scan(&zone); e != nil {
			return e
		}
		o, e := schedule.FutureOccurrence(in.Date, in.Time, zone, in.Duration, s.now())
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,$4::date,$5,$6,$7) RETURNING id::text`, club, in.GroupID, in.TrainerID, in.Date, zone, o.StartsAt, o.EndsAt).Scan(&id)
		if e != nil {
			return e
		}
		out, e = scan(tx.QueryRow(ctx, `SELECT `+columns+joins+`WHERE s.id::text=$1`, id))
		if e != nil {
			return e
		}
		return groups.Audit(ctx, tx, auth.CurrentUser(ctx).ID, club, id, "training_session", "created", nil, state(out))
	})
	if e != nil {
		fail(w, e)
		return
	}
	reply(w, 201, out)
}

type commandInput struct {
	Version     int    `json:"version"`
	Reason      string `json:"reason"`
	ConfirmOpen bool   `json:"confirmOpen"`
	TrainerID   string `json:"trainerId"`
	Date        string `json:"date"`
	Time        string `json:"startsTime"`
	Duration    int    `json:"durationMinutes"`
}

func (s *Service) command(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in commandInput
		if !decode(w, r, &in) {
			return
		}
		in.Reason = strings.TrimSpace(in.Reason)
		valid := in.Version > 0
		switch action {
		case "cancel":
			valid = valid && utf8.ValidString(in.Reason) && utf8.RuneCountInString(in.Reason) >= 1 && utf8.RuneCountInString(in.Reason) <= 1000 && !strings.ContainsFunc(in.Reason, unicode.IsControl) && in.TrainerID == "" && in.Date == "" && in.Time == "" && in.Duration == 0
		case "replace-trainer":
			valid = valid && in.TrainerID != "" && in.Reason == "" && !in.ConfirmOpen && in.Date == "" && in.Time == "" && in.Duration == 0
		case "reschedule":
			valid = valid && in.TrainerID == "" && in.Reason == "" && !in.ConfirmOpen
		}
		if !valid {
			fail(w, schedule.ErrInput)
			return
		}
		var out Session
		e := s.transaction(r, func(tx pgx.Tx) error {
			ctx := r.Context()
			club, id := r.PathValue("clubID"), r.PathValue("sessionID")
			old, e := scan(tx.QueryRow(ctx, `SELECT `+columns+joins+`WHERE s.club_id::text=$1 AND s.id::text=$2 FOR UPDATE OF s`, club, id))
			if e != nil {
				return e
			}
			if old.Version != in.Version {
				return errVersion
			}
			if old.SupersededAt != nil || (old.Status != "planned" && old.Status != "open") {
				return errState
			}
			switch action {
			case "cancel":
				if old.Status == "open" && !in.ConfirmOpen {
					return errConfirm
				}
				// No attendance writes: do not fill unmarked cards with absences.
				_, e = tx.Exec(ctx, `UPDATE pride.training_sessions SET status='cancelled',cancelled_at=now(),cancellation_reason=$2,closed_at=CASE WHEN status='open' THEN coalesce(closed_at,now()) ELSE closed_at END,version=version+1,updated_at=now() WHERE id::text=$1`, id, in.Reason)
			case "replace-trainer":
				if _, e = groups.ReadGroup(ctx, tx, club, old.GroupID, true); e != nil {
					return e
				}
				if e = auth.RequireTrainer(ctx, tx, in.TrainerID, club); e != nil {
					return e
				}
				if old.TrainerID == in.TrainerID {
					out = old
					return nil
				}
				_, e = tx.Exec(ctx, `UPDATE pride.training_sessions SET trainer_id=$2,is_manual_override=true,version=version+1,updated_at=now() WHERE id::text=$1`, id, in.TrainerID)
			case "reschedule":
				if old.Status != "planned" || old.Frozen || !old.StartsAt.After(s.now()) {
					return errState
				}
				if _, e = groups.ReadGroup(ctx, tx, club, old.GroupID, true); e != nil {
					return e
				}
				if e = auth.RequireTrainer(ctx, tx, old.TrainerID, club); e != nil {
					return e
				}
				o, err := schedule.FutureOccurrence(in.Date, in.Time, old.Timezone, in.Duration, s.now())
				if err != nil {
					return err
				}
				_, e = tx.Exec(ctx, `UPDATE pride.training_sessions SET starts_at=$2,ends_at=$3,is_manual_override=true,version=version+1,updated_at=now() WHERE id::text=$1`, id, o.StartsAt, o.EndsAt)
			}
			if e != nil {
				return e
			}
			out, e = scan(tx.QueryRow(ctx, `SELECT `+columns+joins+`WHERE s.id::text=$1`, id))
			if e != nil {
				return e
			}
			return groups.Audit(ctx, tx, auth.CurrentUser(ctx).ID, club, id, "training_session", action, state(old), state(out))
		})
		if e != nil {
			fail(w, e)
			return
		}
		reply(w, 200, out)
	}
}
func (s *Service) canReadAll(r *http.Request) (bool, error) {
	_, e := s.auth.RequireClub(r.Context(), auth.CurrentUser(r.Context()).ID, r.PathValue("clubID"), "administrator")
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	return e == nil, e
}

const groupScope = `($2 OR EXISTS(SELECT 1 FROM pride.training_sessions s WHERE s.group_id=g.id AND s.trainer_id=$3::uuid AND s.superseded_at IS NULL) OR EXISTS(SELECT 1 FROM pride.schedule_rules r WHERE r.group_id=g.id AND r.trainer_id=$3::uuid))`

func (s *Service) listGroups(w http.ResponseWriter, r *http.Request) {
	all, e := s.canReadAll(r)
	if e != nil {
		fail(w, e)
		return
	}
	rows, e := s.pool.Query(r.Context(), `SELECT g.id::text,g.club_id::text,g.name,g.archived_at IS NOT NULL,g.version FROM pride.training_groups g WHERE g.club_id::text=$1 AND `+groupScope+` ORDER BY g.name,g.id`, r.PathValue("clubID"), all, auth.CurrentUser(r.Context()).ID)
	if e != nil {
		fail(w, e)
		return
	}
	defer rows.Close()
	out := []groups.Group{}
	for rows.Next() {
		var g groups.Group
		if e = rows.Scan(&g.ID, &g.ClubID, &g.Name, &g.Archived, &g.Version); e != nil {
			fail(w, e)
			return
		}
		out = append(out, g)
	}
	if e = rows.Err(); e != nil {
		fail(w, e)
		return
	}
	reply(w, 200, out)
}
func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	from, until, group := r.URL.Query().Get("from"), r.URL.Query().Get("until"), r.URL.Query().Get("groupId")
	if !groups.ValidDate(from) || !groups.ValidDate(until) || until <= from {
		fail(w, schedule.ErrInput)
		return
	}
	a, _ := time.Parse("2006-01-02", from)
	b, _ := time.Parse("2006-01-02", until)
	if b.Sub(a) > 366*24*time.Hour {
		fail(w, schedule.ErrInput)
		return
	}
	all, e := s.canReadAll(r)
	if e != nil {
		fail(w, e)
		return
	}
	ctx := r.Context()
	club, user := r.PathValue("clubID"), auth.CurrentUser(ctx).ID
	if group != "" {
		var id string
		e = s.pool.QueryRow(ctx, `SELECT g.id::text FROM pride.training_groups g WHERE g.club_id::text=$1 AND `+groupScope+` AND g.id::text=$4`, club, all, user, group).Scan(&id)
		if e != nil {
			fail(w, e)
			return
		}
	}
	rows, e := s.pool.Query(ctx, `SELECT `+columns+joins+`WHERE s.club_id::text=$1 AND (s.starts_at AT TIME ZONE s.timezone)::date >= $2::date AND (s.starts_at AT TIME ZONE s.timezone)::date < $3::date AND s.superseded_at IS NULL AND ($4 OR s.trainer_id=$5::uuid) AND ($6='' OR s.group_id::text=$6) ORDER BY s.starts_at,s.id`, club, from, until, all, user, group)
	if e != nil {
		fail(w, e)
		return
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		v, e := scan(rows)
		if e != nil {
			fail(w, e)
			return
		}
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		fail(w, e)
		return
	}
	reply(w, 200, out)
}
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	base := "/api/clubs/{clubID}/sessions"
	mux.Handle("GET "+base, s.access(false, s.list))
	mux.Handle("GET "+base+"/groups", s.access(false, s.listGroups))
	mux.Handle("POST "+base, s.access(true, s.create))
	for _, action := range []string{"cancel", "replace-trainer", "reschedule"} {
		mux.Handle("POST "+base+"/{sessionID}/"+action, s.access(true, s.command(action)))
	}
	mux.Handle("/", fallback)
	return mux
}
