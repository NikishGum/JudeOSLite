package participants

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errInput    = errors.New("invalid input")
	errConflict = errors.New("version conflict")
	errIdentity = errors.New("shared identity requires owner")
)

type Card struct {
	ID                 string `json:"id"`
	ParticipantID      string `json:"participantId"`
	ClubID             string `json:"clubId"`
	FullName           string `json:"fullName"`
	BirthDate          string `json:"birthDate"`
	ParticipantVersion int    `json:"participantVersion"`
	JoinedOn           string `json:"joinedOn"`
	ContactKind        string `json:"contactKind"`
	ContactValue       string `json:"contactValue"`
	ContactName        string `json:"contactName"`
	Archived           bool   `json:"archived"`
	Version            int    `json:"version"`
	CanEditIdentity    bool   `json:"canEditIdentity"`
}
type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func New(pool *pgxpool.Pool, access *auth.Service) *Service { return &Service{pool, access} }

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, err error) {
	err = groups.NormalizeError(err)
	switch {
	case errors.Is(err, groups.ErrInput), errors.Is(err, groups.ErrArchived), errors.Is(err, groups.ErrOverlap):
		groups.Fail(w, err)
	case errors.Is(err, errInput):
		platform.WriteError(w, 400, "invalid_input", "Проверьте ФИО, полные даты и контакт. Дата вступления — от рождения до сегодняшнего дня.")
	case errors.Is(err, errConflict):
		platform.WriteError(w, 409, "version_conflict", "Карточка уже изменена. Загрузите актуальную запись и повторите правку.")
	case errors.Is(err, errIdentity):
		platform.WriteError(w, 403, "identity_owner_required", "Общие ФИО и дату рождения меняет владелец. Контакт этого филиала можно изменить отдельно.")
	case errors.Is(err, pgx.ErrNoRows):
		platform.WriteError(w, 404, "not_found", "Карточка или филиал недоступны.")
	case errors.Is(err, clubs.ErrArchived):
		platform.WriteError(w, 409, "club_archived", "Филиал в архиве. Изменения реестра запрещены.")
	default:
		platform.WriteError(w, 503, "not_ready", "Система временно недоступна.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, errInput)
		return false
	}
	return true
}
func text(value string, max int, required bool) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > max || (required && value == "") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func date(value string) (time.Time, bool) {
	d, err := time.Parse("2006-01-02", value)
	return d, err == nil && len(value) == 10 && d.Year() >= 1 && d.Format("2006-01-02") == value
}

type IdentityInput struct {
	FullName  string `json:"fullName"`
	BirthDate string `json:"birthDate"`
	Version   int    `json:"version"`
}
type cardInput struct {
	InitialGroup *groups.InitialGroup `json:"initialGroup,omitempty"`
	Identity     *IdentityInput       `json:"identity,omitempty"`
	JoinedOn     string               `json:"joinedOn"`
	ContactKind  string               `json:"contactKind"`
	ContactValue string               `json:"contactValue"`
	ContactName  string               `json:"contactName"`
	Version      int                  `json:"version"`
}

func (in *cardInput) validate() bool {
	in.ContactValue = strings.TrimSpace(in.ContactValue)
	in.ContactName = strings.TrimSpace(in.ContactName)
	if _, ok := date(in.JoinedOn); !ok {
		return false
	}
	if !text(in.ContactValue, 254, true) || !text(in.ContactName, 200, false) {
		return false
	}
	switch in.ContactKind {
	case "phone":
		digits := 0
		for i, r := range in.ContactValue {
			if r >= '0' && r <= '9' {
				digits++
				continue
			}
			if r == '+' && i == 0 {
				continue
			}
			if !strings.ContainsRune(" ()-", r) {
				return false
			}
		}
		if digits < 7 || digits > 15 {
			return false
		}
	case "email":
		a, err := mail.ParseAddress(in.ContactValue)
		if err != nil || a.Address != in.ContactValue || a.Name != "" {
			return false
		}
	default:
		return false
	}
	if in.Identity != nil {
		in.Identity.FullName = strings.TrimSpace(in.Identity.FullName)
		if !text(in.Identity.FullName, 200, true) {
			return false
		}
		if _, ok := date(in.Identity.BirthDate); !ok {
			return false
		}
	}
	return true
}
func validDates(birth, joined, timezone string) bool {
	b, bok := date(birth)
	j, jok := date(joined)
	loc, err := time.LoadLocation(timezone)
	if !bok || !jok || err != nil {
		return false
	}
	today := time.Now().In(loc).Format("2006-01-02")
	return !j.Before(b) && birth <= today && joined <= today
}

const columns = `m.id::text,m.participant_id::text,m.club_id::text,p.full_name,to_char(p.birth_date,'YYYY-MM-DD'),p.version,
 to_char(m.joined_on,'YYYY-MM-DD'),m.contact_kind,m.contact_value,coalesce(m.contact_name,''),m.archived_at IS NOT NULL,m.version,
 ($3::boolean OR NOT EXISTS(SELECT 1 FROM pride.club_memberships other WHERE other.participant_id=p.id AND other.club_id<>m.club_id))`

func scan(row pgx.Row) (Card, error) {
	var c Card
	err := row.Scan(&c.ID, &c.ParticipantID, &c.ClubID, &c.FullName, &c.BirthDate, &c.ParticipantVersion, &c.JoinedOn, &c.ContactKind, &c.ContactValue, &c.ContactName, &c.Archived, &c.Version, &c.CanEditIdentity)
	return c, err
}
func read(ctx context.Context, tx pgx.Tx, clubID, id string, owner bool) (Card, error) {
	return scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM pride.club_memberships m JOIN pride.participants p ON p.id=m.participant_id WHERE m.club_id::text=$1 AND m.id::text=$2`, clubID, id, owner))
}
func (s *Service) access(next http.HandlerFunc) http.Handler {
	return s.auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.auth.RequireClub(r.Context(), auth.CurrentUser(r.Context()).ID, r.PathValue("clubID"), "administrator"); err != nil {
			fail(w, err)
			return
		}
		next(w, r)
	}))
}

// Recheck live assignments inside the write transaction, locking actor and role
// until commit so disable/revoke cannot race a new authorized operation.
func (s *Service) transaction(r *http.Request, change func(pgx.Tx, string, bool, string) error) error {
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	actor := auth.CurrentUser(ctx).ID
	clubID := r.PathValue("clubID")
	owner, err := auth.LockClubAdministrator(ctx, tx, clubID)
	if err != nil {
		return err
	}
	if err = clubs.RequireActive(ctx, tx, clubID); err != nil {
		return err
	}
	var zone string
	if err = tx.QueryRow(ctx, `SELECT timezone FROM pride.clubs WHERE id::text=$1`, clubID).Scan(&zone); err != nil {
		return err
	}
	if err = change(tx, actor, owner, zone); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Versions, dates and change flags only: never names, birth dates or contacts.
func state(c Card) any {
	return map[string]any{"participantId": c.ParticipantID, "version": c.Version, "participantVersion": c.ParticipantVersion, "joinedOn": c.JoinedOn, "archived": c.Archived}
}
func audit(ctx context.Context, tx pgx.Tx, actor, action string, before *Card, after Card) error {
	var old any
	if before != nil {
		old = state(*before)
	}
	a := map[string]any{"state": state(after)}
	if before != nil {
		a["identityChanged"] = before.FullName != after.FullName || before.BirthDate != after.BirthDate
		a["contactChanged"] = before.ContactKind != after.ContactKind || before.ContactValue != after.ContactValue || before.ContactName != after.ContactName
	}
	b, _ := json.Marshal(old)
	n, _ := json.Marshal(a)
	_, err := tx.Exec(ctx, `INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,before_state,after_state) VALUES($1,$2,'club_membership',$3,$4,$5,$6)`, after.ClubID, actor, after.ID, action, b, n)
	return err
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	filter := r.URL.Query().Get("state")
	if filter == "" {
		filter = "active"
	}
	page := 0
	var err error
	if value := r.URL.Query().Get("page"); value != "" {
		page, err = strconv.Atoi(value)
	}
	if err != nil || page < 0 || page > 100000 || !text(q, 200, false) || (filter != "active" && filter != "archived" && filter != "all") {
		fail(w, errInput)
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT `+columns+` FROM pride.club_memberships m JOIN pride.participants p ON p.id=m.participant_id
 WHERE m.club_id::text=$1 AND ($4='all' OR (m.archived_at IS NOT NULL)=($4='archived'))
 AND ($2='' OR strpos(lower(p.full_name),lower($2))>0 OR strpos(lower(m.contact_value),lower($2))>0 OR strpos(lower(coalesce(m.contact_name,'')),lower($2))>0)
 ORDER BY lower(p.full_name),m.id LIMIT 51 OFFSET $5`, r.PathValue("clubID"), q, auth.CurrentUser(r.Context()).IsOwner, filter, page*50)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	cards := []Card{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			fail(w, err)
			return
		}
		cards = append(cards, c)
	}
	if err = rows.Err(); err != nil {
		fail(w, err)
		return
	}
	more := len(cards) > 50
	if more {
		cards = cards[:50]
	}
	respond(w, 200, map[string]any{"items": cards, "hasMore": more})
}
func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	c, err := scan(s.pool.QueryRow(r.Context(), `SELECT `+columns+` FROM pride.club_memberships m JOIN pride.participants p ON p.id=m.participant_id WHERE m.club_id::text=$1 AND m.id::text=$2`, r.PathValue("clubID"), r.PathValue("membershipID"), auth.CurrentUser(r.Context()).IsOwner))
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, c)
}

// Always lock the identity before its local card. Membership insertion uses the
// same identity lock; count/check/edit therefore observes stable club links.
func lockCard(ctx context.Context, tx pgx.Tx, clubID, id string, owner bool) (Card, error) {
	var participant string
	if err := tx.QueryRow(ctx, `SELECT participant_id::text FROM pride.club_memberships WHERE club_id::text=$1 AND id::text=$2`, clubID, id).Scan(&participant); err != nil {
		return Card{}, err
	}
	var locked string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM pride.participants WHERE id=$1 FOR UPDATE`, participant).Scan(&locked); err != nil {
		return Card{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT id::text FROM pride.club_memberships WHERE club_id::text=$1 AND id::text=$2 FOR UPDATE`, clubID, id).Scan(&locked); err != nil {
		return Card{}, err
	}
	return read(ctx, tx, clubID, id, owner)
}
func (s *Service) save(w http.ResponseWriter, r *http.Request) {
	var in cardInput
	if !decode(w, r, &in) {
		return
	}
	creating := r.Method == http.MethodPost
	if !in.validate() || (creating && (in.Identity == nil || in.Identity.Version != 0 || in.Version != 0)) || (!creating && (in.Version < 1 || in.InitialGroup != nil)) {
		fail(w, errInput)
		return
	}
	var result Card
	err := s.transaction(r, func(tx pgx.Tx, actor string, owner bool, zone string) error {
		ctx := r.Context()
		clubID := r.PathValue("clubID")
		id := r.PathValue("membershipID")
		var before *Card
		action := "created"
		if creating {
			if !validDates(in.Identity.BirthDate, in.JoinedOn, zone) {
				return errInput
			}
			var participant string
			if err := tx.QueryRow(ctx, `INSERT INTO pride.participants(full_name,birth_date) VALUES($1,$2::date) RETURNING id::text`, in.Identity.FullName, in.Identity.BirthDate).Scan(&participant); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value,contact_name) VALUES($1,$2,$3::date,$4,$5,nullif($6,'')) RETURNING id::text`, participant, clubID, in.JoinedOn, in.ContactKind, in.ContactValue, in.ContactName).Scan(&id); err != nil {
				return err
			}
			if in.InitialGroup != nil {
				if err := groups.AddInitial(ctx, tx, actor, clubID, id, *in.InitialGroup); err != nil {
					return err
				}
			}
		} else {
			old, err := lockCard(ctx, tx, clubID, id, owner)
			if err != nil {
				return err
			}
			if old.Version != in.Version {
				return errConflict
			}
			birth := old.BirthDate
			if in.Identity != nil {
				if !old.CanEditIdentity {
					return errIdentity
				}
				if old.ParticipantVersion != in.Identity.Version {
					return errConflict
				}
				birth = in.Identity.BirthDate
				// A shared birth-date edit must remain valid for every local joined_on.
				var invalid bool
				if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pride.club_memberships WHERE participant_id=$1 AND id<>$2 AND joined_on<$3::date)`, old.ParticipantID, old.ID, birth).Scan(&invalid); err != nil {
					return err
				}
				if invalid {
					return errInput
				}
			}
			if !validDates(birth, in.JoinedOn, zone) {
				return errInput
			}
			if in.Identity != nil && (old.FullName != in.Identity.FullName || old.BirthDate != birth) {
				if _, err = tx.Exec(ctx, `UPDATE pride.participants SET full_name=$2,birth_date=$3::date,version=version+1,updated_at=now() WHERE id=$1`, old.ParticipantID, in.Identity.FullName, birth); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, `UPDATE pride.club_memberships SET joined_on=$2::date,contact_kind=$3,contact_value=$4,contact_name=nullif($5,''),version=version+1,updated_at=now() WHERE id=$1`, old.ID, in.JoinedOn, in.ContactKind, in.ContactValue, in.ContactName); err != nil {
				return err
			}
			before = &old
			action = "updated"
		}
		var err error
		result, err = read(ctx, tx, clubID, id, owner)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, action, before, result)
	})
	if err != nil {
		fail(w, err)
		return
	}
	status := 200
	if creating {
		status = 201
	}
	respond(w, status, result)
}
func (s *Service) archive(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version int `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		fail(w, errInput)
		return
	}
	restore := strings.HasSuffix(r.URL.Path, "/restore")
	var result Card
	err := s.transaction(r, func(tx pgx.Tx, actor string, owner bool, _ string) error {
		ctx := r.Context()
		old, err := lockCard(ctx, tx, r.PathValue("clubID"), r.PathValue("membershipID"), owner)
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return errConflict
		}
		if old.Archived != restore {
			result = old
			return nil
		}
		if _, err = tx.Exec(ctx, `UPDATE pride.club_memberships SET archived_at=CASE WHEN $2 THEN NULL ELSE now() END,version=version+1,updated_at=now() WHERE id=$1`, old.ID, restore); err != nil {
			return err
		}
		result, err = read(ctx, tx, old.ClubID, old.ID, owner)
		if err != nil {
			return err
		}
		action := "archived"
		if restore {
			action = "restored"
		}
		return audit(ctx, tx, actor, action, &old, result)
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
func (s *Service) Handler(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/clubs/{clubID}/participants", s.access(s.list))
	mux.Handle("POST /api/clubs/{clubID}/participants", s.access(s.save))
	mux.Handle("GET /api/clubs/{clubID}/participants/{membershipID}", s.access(s.get))
	mux.Handle("PUT /api/clubs/{clubID}/participants/{membershipID}", s.access(s.save))
	mux.Handle("POST /api/clubs/{clubID}/participants/{membershipID}/archive", s.access(s.archive))
	mux.Handle("POST /api/clubs/{clubID}/participants/{membershipID}/restore", s.access(s.archive))
	mux.Handle("/", fallback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/participants") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}
