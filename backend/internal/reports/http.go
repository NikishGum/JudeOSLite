// Package reports computes attendance directly from historical sessions and current marks.
package reports

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var errInput = errors.New("invalid report filters")
var errForbidden = errors.New("owner required")

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }
func (s *Service) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/reports/filters", s.auth.Require(http.HandlerFunc(s.filters)))
	mux.Handle("GET /api/reports/attendance", s.auth.Require(http.HandlerFunc(s.attendance)))
	mux.Handle("/", next)
	return mux
}
func reply(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errForbidden):
		platform.WriteError(w, 403, "forbidden", "Статистика сети доступна только владельцу.")
	case errors.Is(err, errInput):
		platform.WriteError(w, 400, "invalid_input", "Выберите филиал, группу и период: обе даты включительно.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Выбранный филиал или группа недоступны.")
	default:
		platform.WriteError(w, 503, "not_ready", "Статистика временно недоступна. Повторите загрузку.")
	}
}

// Recheck live owner and session under locks before returning even filter names.
func (s *Service) begin(r *http.Request) (pgx.Tx, error) {
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	owner, err := auth.LockSession(r.Context(), tx)
	if err == nil && !owner {
		err = errForbidden
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = errForbidden
	}
	if err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

type Club struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}
type Group struct {
	ID       string `json:"id"`
	ClubID   string `json:"clubId"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}
type Filters struct {
	Clubs  []Club  `json:"clubs"`
	Groups []Group `json:"groups"`
}

func (s *Service) filters(w http.ResponseWriter, r *http.Request) {
	tx, err := s.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	result := Filters{Clubs: []Club{}, Groups: []Group{}}
	rows, err := tx.Query(r.Context(), `SELECT id::text,name,archived_at IS NOT NULL FROM pride.clubs ORDER BY name,id`)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		var c Club
		if err = rows.Scan(&c.ID, &c.Name, &c.Archived); err != nil {
			break
		}
		result.Clubs = append(result.Clubs, c)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, err)
		return
	}
	rows, err = tx.Query(r.Context(), `SELECT id::text,club_id::text,name,archived_at IS NOT NULL FROM pride.training_groups ORDER BY name,club_id,id`)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		var g Group
		if err = rows.Scan(&g.ID, &g.ClubID, &g.Name, &g.Archived); err != nil {
			break
		}
		result.Groups = append(result.Groups, g)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, result)
}

type query struct{ club, group, from, until string }

func parse(values url.Values) (query, error) {
	q := query{strings.ToLower(values.Get("clubId")), strings.ToLower(values.Get("groupId")), values.Get("from"), values.Get("until")}
	for k, v := range values {
		if len(v) != 1 || (k != "clubId" && k != "groupId" && k != "from" && k != "until") {
			return q, errInput
		}
	}
	if (q.club != "" && !uuid.MatchString(q.club)) || (q.group != "" && !uuid.MatchString(q.group)) {
		return q, errInput
	}
	for _, raw := range []string{q.from, q.until} {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil || len(raw) != 10 || d.Year() < 1 {
			return q, errInput
		}
	}
	if q.from > q.until {
		return q, errInput
	}
	return q, nil
}

type Totals struct {
	Closed     int64    `json:"closed"`
	Planned    int64    `json:"planned"`
	Open       int64    `json:"open"`
	Present    int64    `json:"present"`
	Absent     int64    `json:"absent"`
	Unknown    int64    `json:"unknown"`
	Illness    int64    `json:"illness"`
	Excused    int64    `json:"excused"`
	Percentage *float64 `json:"percentage"`
}
type Row struct {
	ClubID        *string `json:"clubId"`
	ClubName      *string `json:"clubName"`
	ClubArchived  bool    `json:"clubArchived"`
	GroupID       *string `json:"groupId"`
	GroupName     *string `json:"groupName"`
	GroupArchived bool    `json:"groupArchived"`
	Totals
}
type Report struct {
	From         string        `json:"from"`
	Until        string        `json:"until"`
	Total        Totals        `json:"total"`
	Clubs        []Row         `json:"clubs"`
	Groups       []Row         `json:"groups"`
	OpenSessions []OpenSession `json:"openSessions"`
}

// Navigation metadata only; no roster, trainer identity or personal marks.
type OpenSession struct {
	ID       string    `json:"id"`
	ClubID   string    `json:"clubId"`
	GroupID  string    `json:"groupId"`
	Date     string    `json:"date"`
	StartsAt time.Time `json:"startsAt"`
	Timezone string    `json:"timezone"`
}

// A single statement gives every level the same MVCC snapshot. Aggregate each
// session BEFORE comparison levels so closed empty rosters count once, and marks
// never multiply session counts. No join to current membership or audit history.
const attendanceSQL = `WITH per_session AS (
 SELECT s.id,s.club_id,s.group_id,s.status,s.starts_at,s.timezone,
 count(r.id) FILTER (WHERE s.status='closed' AND r.state='present') AS present,
 count(r.id) FILTER (WHERE s.status='closed' AND r.state='absent') AS absent,
 count(r.id) FILTER (WHERE s.status='closed' AND r.absence_reason='unknown') AS unknown,
 count(r.id) FILTER (WHERE s.status='closed' AND r.absence_reason='illness') AS illness,
 count(r.id) FILTER (WHERE s.status='closed' AND r.absence_reason='excused') AS excused
 FROM pride.training_sessions s LEFT JOIN pride.attendance_records r ON r.training_session_id=s.id
 WHERE s.superseded_at IS NULL AND s.status IN ('closed','planned','open')
 AND (s.starts_at AT TIME ZONE s.timezone)::date BETWEEN $1::date AND $2::date
 AND ($3='' OR s.club_id::text=$3) AND ($4='' OR s.group_id::text=$4)
 GROUP BY s.id
), levels AS (
 SELECT CASE WHEN GROUPING(c.id)=1 THEN 'network' WHEN GROUPING(g.id)=1 THEN 'club' ELSE 'group' END AS level,
 c.id::text AS club_id,c.name AS club_name,coalesce(c.archived_at IS NOT NULL,false) AS club_archived,
 g.id::text AS group_id,g.name AS group_name,coalesce(g.archived_at IS NOT NULL,false) AS group_archived,
 count(s.id) FILTER (WHERE s.status='closed') AS closed,
 count(s.id) FILTER (WHERE s.status='planned') AS planned,
 count(s.id) FILTER (WHERE s.status='open') AS open,
 coalesce(sum(s.present),0)::bigint AS present,coalesce(sum(s.absent),0)::bigint AS absent,
 coalesce(sum(s.unknown),0)::bigint AS unknown,coalesce(sum(s.illness),0)::bigint AS illness,coalesce(sum(s.excused),0)::bigint AS excused
 FROM pride.clubs c LEFT JOIN pride.training_groups g ON g.club_id=c.id
 LEFT JOIN per_session s ON s.group_id=g.id AND s.club_id=c.id
 WHERE ($3='' OR c.id::text=$3) AND ($4='' OR g.id::text=$4)
 GROUP BY GROUPING SETS ((c.id,c.name,c.archived_at,g.id,g.name,g.archived_at),(c.id,c.name,c.archived_at),())
 HAVING GROUPING(g.id)=1 OR g.id IS NOT NULL
)
SELECT *,present::double precision*100/nullif(present+absent,0) AS percentage,
 CASE WHEN level='network' THEN coalesce((SELECT jsonb_agg(jsonb_build_object(
 'id',id,'clubId',club_id,'groupId',group_id,
 'date',to_char(starts_at AT TIME ZONE timezone,'YYYY-MM-DD'),'startsAt',starts_at,'timezone',timezone)
 ORDER BY starts_at,id) FROM per_session WHERE status='open'),'[]'::jsonb) ELSE '[]'::jsonb END AS open_sessions
FROM levels ORDER BY level,club_name,club_id,group_name,group_id`

func (s *Service) attendance(w http.ResponseWriter, r *http.Request) {
	tx, err := s.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(context.Background())
	q, err := parse(r.URL.Query())
	if err != nil {
		fail(w, err)
		return
	}
	var exists bool
	if q.club != "" {
		err = tx.QueryRow(r.Context(), `SELECT true FROM pride.clubs WHERE id::text=$1`, q.club).Scan(&exists)
	}
	if err == nil && q.group != "" {
		err = tx.QueryRow(r.Context(), `SELECT true FROM pride.training_groups WHERE id::text=$1 AND ($2='' OR club_id::text=$2)`, q.group, q.club).Scan(&exists)
	}
	if err != nil {
		fail(w, err)
		return
	}
	result := Report{From: q.from, Until: q.until, Clubs: []Row{}, Groups: []Row{}, OpenSessions: []OpenSession{}}
	rows, err := tx.Query(r.Context(), attendanceSQL, q.from, q.until, q.club, q.group)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		var level string
		var v Row
		var opened []OpenSession
		err = rows.Scan(&level, &v.ClubID, &v.ClubName, &v.ClubArchived, &v.GroupID, &v.GroupName, &v.GroupArchived, &v.Closed, &v.Planned, &v.Open, &v.Present, &v.Absent, &v.Unknown, &v.Illness, &v.Excused, &v.Percentage, &opened)
		if err != nil {
			break
		}
		switch level {
		case "network":
			result.Total = v.Totals
			result.OpenSessions = opened
		case "club":
			result.Clubs = append(result.Clubs, v)
		case "group":
			result.Groups = append(result.Groups, v)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, result)
}
