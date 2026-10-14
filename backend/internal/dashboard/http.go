// Package dashboard reads the live working panel without changing domain state.
package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
	now  func() time.Time
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access, time.Now} }
func (s *Service) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/dashboard", s.auth.Require(http.HandlerFunc(s.read)))
	mux.Handle("/", next)
	return mux
}

type Club struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Timezone  string `json:"timezone"`
	Today     string `json:"today"`
	CanManage bool   `json:"canManage"`
}
type Counters struct {
	// Trainer scope never exposes a branch-wide registry count.
	Participants *int64 `json:"participants"`
	Groups       int64  `json:"groups"`
	Today        int64  `json:"today"`
	Open         int64  `json:"open"`
}
type Session struct {
	ID          string    `json:"id"`
	ClubID      string    `json:"clubId"`
	GroupID     string    `json:"groupId"`
	GroupName   string    `json:"groupName"`
	TrainerName string    `json:"trainerName"`
	Timezone    string    `json:"timezone"`
	Date        string    `json:"date"`
	BranchDate  string    `json:"branchDate"`
	StartsAt    time.Time `json:"startsAt"`
	EndsAt      time.Time `json:"endsAt"`
	Status      string    `json:"status"`
	CanMark     bool      `json:"canMark"`
	Roster      int64     `json:"roster"`
	Marked      int64     `json:"marked"`
}
type Group struct {
	ID      string `json:"id"`
	ClubID  string `json:"clubId"`
	Name    string `json:"name"`
	Members int64  `json:"members"`
}
type Panel struct {
	Mode       string    `json:"mode"`
	ReadAt     time.Time `json:"readAt"`
	Clubs      []Club    `json:"clubs"`
	Counters   Counters  `json:"counters"`
	Next       *Session  `json:"next"`
	Today      []Session `json:"today"`
	Unfinished []Session `json:"unfinished"`
	Groups     []Group   `json:"groups"`
}

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// These scopes match the existing session/group read contract. IDs below come
// only from locked live roles, never from client claims about a role or actor.
const assignedGroup = `(EXISTS(SELECT 1 FROM pride.schedule_rules r WHERE r.group_id=g.id AND r.trainer_id=$3::uuid)
 OR EXISTS(SELECT 1 FROM pride.training_sessions a WHERE a.group_id=g.id AND a.trainer_id=$3::uuid AND a.superseded_at IS NULL))`
const sessionScope = ` FROM pride.training_sessions s JOIN pride.clubs c ON c.id=s.club_id
 JOIN pride.training_groups g ON g.id=s.group_id JOIN pride.staff_users u ON u.id=s.trainer_id
 LEFT JOIN LATERAL (SELECT count(*) AS roster,count(*) FILTER(WHERE state IS NOT NULL) AS marked
   FROM pride.attendance_records WHERE training_session_id=s.id) marks ON true
 WHERE s.club_id=ANY($1::uuid[]) AND s.superseded_at IS NULL
 AND (s.club_id=ANY($2::uuid[]) OR s.trainer_id=$3::uuid)`
const sessionColumns = `SELECT s.id::text,s.club_id::text,s.group_id::text,g.name,u.display_name,s.timezone,
 (s.starts_at AT TIME ZONE s.timezone)::date::text,(s.starts_at AT TIME ZONE c.timezone)::date::text,
 s.starts_at,s.ends_at,s.status,(s.trainer_id=$3::uuid AND s.club_id=ANY($5::uuid[])),marks.roster,marks.marked`

func readSessions(ctx context.Context, tx pgx.Tx, condition string, args []any) ([]Session, error) {
	rows, err := tx.Query(ctx, sessionColumns+sessionScope+condition, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Session{}
	for rows.Next() {
		var v Session
		if err := rows.Scan(&v.ID, &v.ClubID, &v.GroupID, &v.GroupName, &v.TrainerName, &v.Timezone, &v.Date, &v.BranchDate, &v.StartsAt, &v.EndsAt, &v.Status, &v.CanMark, &v.Roster, &v.Marked); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *Service) panel(r *http.Request, selected string) (Panel, error) {
	ctx := r.Context()
	// All counters and limited lists use one database snapshot. FOR SHARE locks
	// retain the live actor/session/roles through the read; this is not a cache.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return Panel{}, err
	}
	defer tx.Rollback(context.Background())
	owner, err := auth.LockSession(ctx, tx)
	if err != nil {
		return Panel{}, err
	}
	actor := auth.CurrentUser(ctx).ID
	roles := map[string]map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT r.club_id::text,r.role FROM pride.staff_club_roles r JOIN pride.clubs c ON c.id=r.club_id WHERE r.staff_user_id=$1 AND c.archived_at IS NULL ORDER BY r.club_id,r.role FOR SHARE OF r`, actor)
	if err != nil {
		return Panel{}, err
	}
	for rows.Next() {
		var club, role string
		if err = rows.Scan(&club, &role); err != nil {
			break
		}
		if roles[club] == nil {
			roles[club] = map[string]bool{}
		}
		roles[club][role] = true
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return Panel{}, err
	}
	result := Panel{Mode: "trainer", ReadAt: s.now().UTC(), Clubs: []Club{}, Today: []Session{}, Unfinished: []Session{}, Groups: []Group{}}
	if owner {
		result.Mode = "owner"
	} else {
		for id, assignment := range roles {
			if (selected == "" || id == selected) && assignment["administrator"] {
				result.Mode = "administrator"
			}
		}
	}
	rows, err = tx.Query(ctx, `SELECT id::text,name,timezone,($3::timestamptz AT TIME ZONE timezone)::date::text
 FROM pride.clubs WHERE archived_at IS NULL AND ($2='' OR id::text=$2)
 AND ($1 OR id::text=ANY($4::text[])) ORDER BY name,id FOR SHARE`, owner, selected, result.ReadAt, roleIDs(roles))
	if err != nil {
		return Panel{}, err
	}
	ids, managers, trainers := []string{}, []string{}, []string{}
	for rows.Next() {
		var club Club
		if err = rows.Scan(&club.ID, &club.Name, &club.Timezone, &club.Today); err != nil {
			break
		}
		club.CanManage = owner || roles[club.ID]["administrator"]
		// An unselected multi-role home follows the RD-01 administrator priority.
		if result.Mode == "administrator" && !club.CanManage {
			continue
		}
		result.Clubs = append(result.Clubs, club)
		ids = append(ids, club.ID)
		if club.CanManage {
			managers = append(managers, club.ID)
		}
		if roles[club.ID]["trainer"] {
			trainers = append(trainers, club.ID)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return Panel{}, err
	}
	if selected != "" && len(ids) == 0 {
		return Panel{}, pgx.ErrNoRows
	}
	if result.Mode != "trainer" {
		var zero int64
		result.Counters.Participants = &zero
	}
	if len(ids) > 0 {
		args := []any{ids, managers, actor, result.ReadAt, trainers}
		var participants int64
		err = tx.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM pride.club_memberships WHERE club_id=ANY($2::uuid[]) AND archived_at IS NULL),
 (SELECT count(*) FROM pride.training_groups g WHERE g.club_id=ANY($1::uuid[]) AND g.archived_at IS NULL AND (g.club_id=ANY($2::uuid[]) OR `+assignedGroup+`)),
 (SELECT count(*) FROM pride.training_sessions s JOIN pride.clubs c ON c.id=s.club_id WHERE s.club_id=ANY($1::uuid[]) AND s.superseded_at IS NULL AND (s.club_id=ANY($2::uuid[]) OR s.trainer_id=$3::uuid) AND (s.starts_at AT TIME ZONE c.timezone)::date=($4::timestamptz AT TIME ZONE c.timezone)::date),
 (SELECT count(*) FROM pride.training_sessions s WHERE s.club_id=ANY($1::uuid[]) AND s.superseded_at IS NULL AND (s.club_id=ANY($2::uuid[]) OR s.trainer_id=$3::uuid) AND s.status='open')`, args[:4]...).Scan(&participants, &result.Counters.Groups, &result.Counters.Today, &result.Counters.Open)
		if err != nil {
			return Panel{}, err
		}
		if result.Counters.Participants != nil {
			*result.Counters.Participants = participants
		}
		next, err := readSessions(ctx, tx, ` AND (s.status='open' OR (s.status='planned' AND (s.starts_at AT TIME ZONE c.timezone)::date>=($4::timestamptz AT TIME ZONE c.timezone)::date)) ORDER BY (s.status='open') DESC,s.starts_at,s.id LIMIT 1`, args)
		if err != nil {
			return Panel{}, err
		}
		if len(next) > 0 {
			result.Next = &next[0]
		}
		result.Today, err = readSessions(ctx, tx, ` AND (s.starts_at AT TIME ZONE c.timezone)::date=($4::timestamptz AT TIME ZONE c.timezone)::date ORDER BY s.starts_at,s.id LIMIT 12`, args)
		if err != nil {
			return Panel{}, err
		}
		result.Unfinished, err = readSessions(ctx, tx, ` AND s.status='open' AND $4::timestamptz IS NOT NULL ORDER BY s.starts_at,s.id LIMIT 6`, args)
		if err != nil {
			return Panel{}, err
		}
		rows, err = tx.Query(ctx, `SELECT g.id::text,g.club_id::text,g.name,
 (SELECT count(*) FROM pride.group_memberships m JOIN pride.club_memberships cm ON cm.id=m.club_membership_id
 WHERE m.group_id=g.id AND cm.archived_at IS NULL AND m.valid_from<=($4::timestamptz AT TIME ZONE c.timezone)::date AND (m.valid_until IS NULL OR m.valid_until>($4::timestamptz AT TIME ZONE c.timezone)::date))
 FROM pride.training_groups g JOIN pride.clubs c ON c.id=g.club_id WHERE g.club_id=ANY($1::uuid[]) AND g.archived_at IS NULL AND (g.club_id=ANY($2::uuid[]) OR `+assignedGroup+`) ORDER BY c.name,g.name,g.id LIMIT 8`, args[:4]...)
		if err != nil {
			return Panel{}, err
		}
		for rows.Next() {
			var g Group
			if err = rows.Scan(&g.ID, &g.ClubID, &g.Name, &g.Members); err != nil {
				break
			}
			result.Groups = append(result.Groups, g)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return Panel{}, err
		}
	}
	return result, tx.Commit(ctx)
}

func roleIDs(roles map[string]map[string]bool) []string {
	result := []string{}
	for id := range roles {
		result = append(result, id)
	}
	return result
}
func (s *Service) read(w http.ResponseWriter, r *http.Request) {
	selected := r.URL.Query().Get("clubId")
	if selected != "" && !uuid.MatchString(selected) {
		platform.WriteError(w, 400, "invalid_input", "Выберите доступный филиал.")
		return
	}
	panel, err := s.panel(r, selected)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			platform.WriteError(w, 404, "not_found", "Филиал или назначение больше недоступны.")
		} else {
			platform.WriteError(w, 503, "not_ready", "Рабочая панель временно недоступна. Повторите загрузку.")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(panel)
}
