package schedule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
	now  func() time.Time
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access, time.Now} }
func reply(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, e error) {
	var p *pgconn.PgError
	if errors.As(e, &p) {
		switch p.Code {
		case "23505":
			e = ErrDuplicate
		case "23P01":
			e = ErrConflict
		case "23514":
			e = ErrInput
		case "23503", "22P02":
			e = pgx.ErrNoRows
		}
	}
	switch {
	case errors.Is(e, ErrInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте режим, даты, время, длительность и выбранные группу/тренера. Конец периода исключён. Разовые и выбранные даты должны быть в будущем; неоднозначное время смены часового пояса недоступно.")
	case errors.Is(e, ErrDuplicate):
		platform.WriteError(w, 409, "session_duplicate", "У группы уже есть занятие в это время, включая отменённое. Выберите другое время.")
	case errors.Is(e, ErrConflict):
		platform.WriteError(w, 409, "version_conflict", "Правило уже изменено. Загрузите актуальную версию.")
	case errors.Is(e, ErrPreview):
		platform.WriteError(w, 409, "preview_changed", "Расписание изменилось после предпросмотра. Проверьте новый предпросмотр и подтвердите снова.")
	case errors.Is(e, clubs.ErrArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Создание и изменение расписания недоступны.")
	case errors.Is(e, groups.ErrArchived):
		platform.WriteError(w, 409, "group_archived", "Группа в архиве. Создание и изменение расписания недоступны.")
	case errors.Is(e, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Правило, группа, тренер или филиал недоступны.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, in any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	d.DisallowUnknownFields()
	if d.Decode(in) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, ErrInput)
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

// Serialize generation, previews and rule changes across all Go processes. This
// is a small single database job, not a separate worker/queue service.
func lock(ctx context.Context, tx pgx.Tx) error {
	_, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(155007)`)
	return e
}
func (s *Service) transaction(r *http.Request, fn func(pgx.Tx) error, commit bool) error {
	ctx := r.Context()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = auth.LockClubAdministrator(ctx, tx, r.PathValue("clubID")); e != nil {
		return e
	}
	if e = lock(ctx, tx); e != nil {
		return e
	}
	if e = clubs.RequireActive(ctx, tx, r.PathValue("clubID")); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	if commit {
		return tx.Commit(ctx)
	}
	return nil
}

const ruleColumns = `id::text,club_id::text,group_id::text,series_id::text,trainer_id::text,mode,coalesce(weekdays,'{}'),coalesce(day_parity,''),coalesce(ARRAY(SELECT to_char(d,'YYYY-MM-DD') FROM unnest(selected_dates) d),'{}'),to_char(starts_time,'HH24:MI'),duration_minutes,timezone,to_char(valid_from,'YYYY-MM-DD'),to_char(valid_until,'YYYY-MM-DD'),version`

func scanRule(row pgx.Row) (Rule, error) {
	var v Rule
	e := row.Scan(&v.ID, &v.ClubID, &v.GroupID, &v.SeriesID, &v.TrainerID, &v.Mode, &v.Weekdays, &v.Parity, &v.Dates, &v.Time, &v.Duration, &v.Timezone, &v.From, &v.Until, &v.Version)
	return v, e
}

const sessionColumns = `s.id::text,s.club_id::text,s.group_id::text,s.schedule_rule_id::text,s.trainer_id::text,to_char(s.starts_on,'YYYY-MM-DD'),s.timezone,s.starts_at,s.ends_at,s.status,s.is_manual_override,s.roster_frozen_at IS NOT NULL,s.version`

func scanSession(row pgx.Row) (Session, error) {
	var v Session
	e := row.Scan(&v.ID, &v.ClubID, &v.GroupID, &v.RuleID, &v.TrainerID, &v.Date, &v.Timezone, &v.StartsAt, &v.EndsAt, &v.Status, &v.Manual, &v.Frozen, &v.Version)
	return v, e
}
func sessionRows(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]Session, error) {
	rows, e := tx.Query(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		v, e := scanSession(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) preview(ctx context.Context, tx pgx.Tx, club, id string, in Input, now time.Time) (Preview, Rule, error) {
	out := Preview{Dates: []Occurrence{}, Affected: []Session{}, Preserved: []Session{}}
	var old Rule
	if _, e := groups.ReadGroup(ctx, tx, club, in.GroupID, true); e != nil {
		return out, old, e
	}
	if e := auth.RequireTrainer(ctx, tx, in.TrainerID, club); e != nil {
		return out, old, e
	}
	if e := tx.QueryRow(ctx, `SELECT timezone FROM pride.clubs WHERE id=$1`, club).Scan(&out.Timezone); e != nil {
		return out, old, e
	}
	loc, e := time.LoadLocation(out.Timezone)
	if e != nil {
		return out, old, e
	}
	if id != "" {
		old, e = scanRule(tx.QueryRow(ctx, `SELECT `+ruleColumns+` FROM pride.schedule_rules WHERE club_id::text=$1 AND id::text=$2 FOR UPDATE`, club, id))
		if e != nil {
			return out, old, e
		}
		if old.Version != in.Version {
			return out, old, ErrConflict
		}
		var later bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pride.schedule_rules WHERE series_id=$1 AND valid_from>$2::date)`, old.SeriesID, old.From).Scan(&later)
		if e != nil {
			return out, old, e
		}
		if later {
			return out, old, ErrConflict
		}
		if in.Mode == "once" || in.GroupID != old.GroupID || in.From <= old.From || in.From < now.In(loc).Format("2006-01-02") || (old.Until != nil && in.From >= *old.Until) {
			return out, old, ErrInput
		}
		existing, e := sessionRows(ctx, tx, `SELECT `+sessionColumns+` FROM pride.training_sessions s JOIN pride.schedule_rules r ON r.id=s.schedule_rule_id WHERE r.series_id=$1 AND s.superseded_at IS NULL AND s.starts_on>=$2::date ORDER BY s.starts_at,s.id FOR UPDATE OF s`, old.SeriesID, in.From)
		if e != nil {
			return out, old, e
		}
		for _, v := range existing {
			if v.Status == "planned" && !v.Manual && !v.Frozen && v.StartsAt.After(now) {
				out.Affected = append(out.Affected, v)
			} else {
				out.Preserved = append(out.Preserved, v)
			}
		}
	} else if in.Version != 0 {
		return out, old, ErrInput
	}
	proposed, e := expand(in, out.Timezone, now)
	if e != nil {
		return out, old, e
	}
	if id == "" && (in.Mode == "date_list" || in.Mode == "once") && len(proposed) != len(in.Dates) {
		return out, old, ErrInput
	}
	if id != "" && in.Mode == "date_list" {
		for _, d := range in.Dates {
			o, e := occurrence(d, in.Time, out.Timezone, in.Duration)
			if e != nil || !o.StartsAt.After(now) {
				return out, old, ErrInput
			}
		}
	}
	affected := map[string]bool{}
	for _, v := range out.Affected {
		affected[v.ID] = true
	}
	preserved := map[string]bool{}
	for _, v := range out.Preserved {
		preserved[v.Date] = true
	}
	for _, o := range proposed {
		if preserved[o.Date] {
			continue
		}
		var slot string
		e = tx.QueryRow(ctx, `SELECT id::text FROM pride.training_sessions WHERE group_id=$1 AND starts_at=$2 AND superseded_at IS NULL`, in.GroupID, o.StartsAt).Scan(&slot)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, old, e
		}
		if e == nil && !affected[slot] {
			return out, old, ErrDuplicate
		}
		out.Dates = append(out.Dates, o)
	}
	in.PreviewToken = ""
	raw, _ := json.Marshal(struct {
		Input   Input
		Old     Rule
		Preview Preview
	}{in, old, out})
	sum := sha256.Sum256(raw)
	out.Token = hex.EncodeToString(sum[:])
	return out, old, nil
}
func insertRule(ctx context.Context, tx pgx.Tx, club, series string, in Input, zone string) (Rule, error) {
	var weekdays any
	var parity any
	var dates any
	if in.Mode == "weekly" {
		weekdays = in.Weekdays
	}
	if in.Mode == "day_parity" {
		parity = in.Parity
	}
	if in.Mode == "date_list" {
		dates = in.Dates
	}
	return scanRule(tx.QueryRow(ctx, `INSERT INTO pride.schedule_rules(club_id,group_id,series_id,trainer_id,mode,weekdays,day_parity,selected_dates,starts_time,duration_minutes,timezone,valid_from,valid_until) VALUES($1,$2,coalesce($3::uuid,gen_random_uuid()),$4,$5,$6,$7,$8::date[],$9::time,$10,$11,$12::date,$13::date) RETURNING `+ruleColumns, club, in.GroupID, nullable(series), in.TrainerID, in.Mode, weekdays, parity, dates, in.Time, in.Duration, zone, in.From, in.Until))
}
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func insertSession(ctx context.Context, tx pgx.Tx, club, rule string, in Input, zone string, o Occurrence) (Session, error) {
	return scanSession(tx.QueryRow(ctx, `INSERT INTO pride.training_sessions AS s(club_id,group_id,schedule_rule_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,$4,$5::date,$6,$7,$8) RETURNING `+sessionColumns, club, in.GroupID, nullable(rule), in.TrainerID, o.Date, zone, o.StartsAt, o.EndsAt))
}
func ruleState(v Rule) any {
	return map[string]any{"seriesId": v.SeriesID, "groupId": v.GroupID, "trainerId": v.TrainerID, "mode": v.Mode, "weekdays": v.Weekdays, "dayParity": v.Parity, "selectedDates": v.Dates, "startsTime": v.Time, "durationMinutes": v.Duration, "timezone": v.Timezone, "from": v.From, "until": v.Until, "version": v.Version}
}
func (s *Service) save(previewOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if !decode(w, r, &in) {
			return
		}
		if e := validate(&in); e != nil {
			fail(w, e)
			return
		}
		var out Preview
		var result any
		e := s.transaction(r, func(tx pgx.Tx) error {
			ctx := r.Context()
			club := r.PathValue("clubID")
			actor := auth.CurrentUser(ctx).ID
			var old Rule
			var e error
			out, old, e = s.preview(ctx, tx, club, r.PathValue("ruleID"), in, s.now())
			if e != nil {
				return e
			}
			if previewOnly {
				return nil
			}
			if in.PreviewToken != out.Token {
				return ErrPreview
			}
			if old.ID != "" {
				if _, e = tx.Exec(ctx, `UPDATE pride.schedule_rules SET valid_until=$2::date,version=version+1,updated_at=now() WHERE id=$1`, old.ID, in.From); e != nil {
					return e
				}
				for _, v := range out.Affected {
					if _, e = tx.Exec(ctx, `UPDATE pride.training_sessions SET superseded_at=now(),version=version+1,updated_at=now() WHERE id=$1`, v.ID); e != nil {
						return e
					}
					if e = groups.Audit(ctx, tx, actor, club, v.ID, "training_session", "superseded", map[string]any{"version": v.Version}, map[string]any{"version": v.Version + 1}); e != nil {
						return e
					}
				}
				if e = groups.Audit(ctx, tx, actor, club, old.ID, "schedule_rule", "ended", ruleState(old), map[string]any{"until": in.From, "version": old.Version + 1}); e != nil {
					return e
				}
			}
			var rule Rule
			if in.Mode != "once" {
				rule, e = insertRule(ctx, tx, club, old.SeriesID, in, out.Timezone)
				if e != nil {
					return e
				}
				if e = groups.Audit(ctx, tx, actor, club, rule.ID, "schedule_rule", "created", nil, ruleState(rule)); e != nil {
					return e
				}
				result = rule
			}
			for _, o := range out.Dates {
				v, e := insertSession(ctx, tx, club, rule.ID, in, out.Timezone, o)
				if e != nil {
					return e
				}
				if in.Mode == "once" {
					result = v
					if e = groups.Audit(ctx, tx, actor, club, v.ID, "training_session", "created", nil, v); e != nil {
						return e
					}
				}
			}
			return nil
		}, !previewOnly)
		if e != nil {
			fail(w, e)
			return
		}
		if previewOnly {
			reply(w, 200, out)
		} else {
			reply(w, 201, result)
		}
	}
}
func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	club := r.PathValue("clubID")
	user := auth.CurrentUser(ctx)
	// Re-read administrator access; a trainer sees only their own assigned rules.
	_, adminErr := s.auth.RequireClub(ctx, user.ID, club, "administrator")
	if adminErr != nil && !errors.Is(adminErr, pgx.ErrNoRows) {
		fail(w, adminErr)
		return
	}
	all := adminErr == nil
	rows, e := s.pool.Query(ctx, `SELECT `+ruleColumns+` FROM pride.schedule_rules WHERE club_id::text=$1 AND ($2 OR trainer_id=$3::uuid) ORDER BY valid_from DESC,created_at DESC,id`, club, all, user.ID)
	if e != nil {
		fail(w, e)
		return
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		v, e := scanRule(rows)
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
func (s *Service) sessions(w http.ResponseWriter, r *http.Request) {
	from, until := r.URL.Query().Get("from"), r.URL.Query().Get("until")
	if !groups.ValidDate(from) || !groups.ValidDate(until) || until <= from {
		fail(w, ErrInput)
		return
	}
	a, _ := time.Parse("2006-01-02", from)
	b, _ := time.Parse("2006-01-02", until)
	if b.Sub(a) > 366*24*time.Hour {
		fail(w, ErrInput)
		return
	}
	ctx := r.Context()
	club := r.PathValue("clubID")
	user := auth.CurrentUser(ctx)
	_, adminErr := s.auth.RequireClub(ctx, user.ID, club, "administrator")
	if adminErr != nil && !errors.Is(adminErr, pgx.ErrNoRows) {
		fail(w, adminErr)
		return
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback(context.Background())
	out, e := sessionRows(ctx, tx, `SELECT `+sessionColumns+` FROM pride.training_sessions s WHERE s.club_id::text=$1 AND s.starts_on>=$2::date AND s.starts_on<$3::date AND s.superseded_at IS NULL AND ($4 OR s.trainer_id=$5::uuid) ORDER BY s.starts_at,s.id`, club, from, until, adminErr == nil, user.ID)
	if e != nil {
		fail(w, e)
		return
	}
	reply(w, 200, out)
}
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	base := "/api/clubs/{clubID}/schedule"
	mux.Handle("GET "+base, s.access(false, s.list))
	mux.Handle("GET "+base+"/sessions", s.access(false, s.sessions))
	mux.Handle("POST "+base+"/preview", s.access(true, s.save(true)))
	mux.Handle("POST "+base, s.access(true, s.save(false)))
	mux.Handle("POST "+base+"/{ruleID}/preview", s.access(true, s.save(true)))
	mux.Handle("PUT "+base+"/{ruleID}", s.access(true, s.save(false)))
	mux.Handle("/", fallback)
	return mux
}
