// Package demodata loads an explicitly requested, fictional Belarusian demo network.
// It is used only by the local seed-demo command, never by server startup.
package demodata

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dataset = "pride-belarus-demo-v1"

var clubs = []struct{ name, address string }{
	{"Минск · Уручье", "г. Минск, ул. Шафарнянская, д. 11"},
	{"Брест · Восток", "г. Брест, ул. Московская, д. 275"},
	{"Гродно · Девятовка", "г. Гродно, ул. Лиможа, д. 32"},
}

type Account struct {
	Login  string `json:"login"`
	Name   string `json:"fullName"`
	Role   string `json:"role"`
	Club   string `json:"club,omitempty"`
	Family string `json:"family,omitempty"`
}
type Result struct {
	Date           string    `json:"date"`
	From           string    `json:"from"`
	Until          string    `json:"until"`
	AlreadyLoaded  bool      `json:"alreadyLoaded"`
	Clubs          int       `json:"clubs"`
	Trainers       int       `json:"trainers"`
	Administrators int       `json:"administrators"`
	Groups         int       `json:"groups"`
	Children       int       `json:"children"`
	Families       int       `json:"families"`
	Parents        int       `json:"parents"`
	Sessions       int       `json:"sessions,omitempty"`
	ClosedSessions int       `json:"closedSessions,omitempty"`
	Payments       int       `json:"payments"`
	Subscriptions  int       `json:"subscriptions"`
	Examples       []Account `json:"exampleAccounts"`
}

func id(kind string, index int) string {
	b := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%d", dataset, kind, index)))
	b[6] = (b[6] & 15) | 80
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func day(d time.Time) string { return d.Format(time.DateOnly) }
func familyName(i int) string {
	return "Семья " + strings.Split(surnames[i%len(surnames)], "|")[2]
}
func staff(i int, role string) person {
	if role == "administrator" {
		return administrators[i]
	}
	return trainers[i]
}
func owner() person {
	return person{"Рудаков Павел Сергеевич", "owner.pavel.rudakov"}
}

// Accounts is deterministic and contains no passwords or database data.
func Accounts() []Account {
	o := owner()
	result := []Account{{Login: o.login, Name: o.name, Role: "owner"}}
	for c, club := range clubs {
		for i := 0; i < 5; i++ {
			p := staff(c*5+i, "trainer")
			result = append(result, Account{Login: p.login, Name: p.name, Role: "trainer", Club: club.name})
		}
		for i := 0; i < 2; i++ {
			p := staff(c*2+i, "administrator")
			result = append(result, Account{Login: p.login, Name: p.name, Role: "administrator", Club: club.name})
		}
		for f := c * 150; f < (c+1)*150; f++ {
			for _, female := range []bool{false, true} {
				p := adult(f, female, "parent")
				result = append(result, Account{Login: p.login, Name: p.name, Role: "parent", Club: club.name, Family: familyName(f)})
			}
		}
	}
	return result
}
func Preview(date string) (Result, error) {
	loc, err := time.LoadLocation("Europe/Minsk")
	if err != nil {
		return Result{}, err
	}
	d, err := time.ParseInLocation(time.DateOnly, date, loc)
	if err != nil || d.Year() < 2000 || d.Year() > 2100 {
		return Result{}, errors.New("date must be YYYY-MM-DD between 2000 and 2100")
	}
	r := Result{Date: date, From: day(d.AddDate(0, 0, -42)), Until: day(d.AddDate(0, 0, 28)), Clubs: 3, Trainers: 15, Administrators: 6, Groups: 30, Children: 600, Families: 450, Parents: 900, Payments: 1800, Subscriptions: 1800}
	a := Accounts()
	r.Examples = []Account{a[0], a[1], a[6], a[8], a[9]}
	return r, nil
}

type loader struct {
	ctx context.Context
	tx  pgx.Tx
	err error
}

func (l *loader) exec(query string, args ...any) {
	if l.err != nil {
		return
	}
	_, l.err = l.tx.Exec(l.ctx, query, args...)
}

// Seed commits the entire dataset and its audit marker atomically. A transaction
// lock serializes concurrent runs; the marker makes retries preserve all edits.
func Seed(ctx context.Context, pool *pgxpool.Pool, date, password string) (Result, error) {
	result, err := Preview(date)
	if err != nil {
		return result, err
	}
	if err = migrations.Check(ctx, pool); err != nil {
		return result, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(context.Background())
	l := loader{ctx: ctx, tx: tx}
	l.exec("SELECT pg_advisory_xact_lock(210013)")
	if l.err != nil {
		return result, l.err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT after_state FROM pride.audit_events WHERE id=$1 AND entity_type='demo_dataset'`, id("marker", 0)).Scan(&raw)
	if err == nil {
		if err = json.Unmarshal(raw, &result); err != nil {
			return result, err
		}
		result.AlreadyLoaded = true
		return result, nil
	}
	if err != pgx.ErrNoRows {
		return result, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return result, err
	}
	// A shared, explicitly chosen password is intentional for these demo accounts.
	// It is never stored in a manifest, audit record, command argument or source.
	o := owner()
	l.user(id("owner", 0), o, hash, true)
	loc, _ := time.LoadLocation("Europe/Minsk")
	anchor, _ := time.ParseInLocation(time.DateOnly, date, loc)
	from := anchor.AddDate(0, 0, -42)
	until := anchor.AddDate(0, 0, 28)
	for c, club := range clubs {
		cid := id("club", c)
		l.exec(`INSERT INTO pride.clubs(id,name,address,timezone) VALUES($1,$2,$3,'Europe/Minsk')`, cid, club.name, club.address)
		for t := 0; t < 5; t++ {
			uid := id("trainer", c*5+t)
			l.user(uid, staff(c*5+t, "trainer"), hash, false)
			l.exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, uid, cid)
		}
		for a := 0; a < 2; a++ {
			uid := id("admin", c*2+a)
			l.user(uid, staff(c*2+a, "administrator"), hash, false)
			l.exec(`INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'administrator')`, uid, cid)
		}
		for p, plan := range []struct {
			name           string
			amount, visits int
		}{{"Базовый · 8 занятий", 8000, 8}, {"Стандарт · 12 занятий", 11000, 12}, {"Интенсив · 16 занятий", 14000, 16}} {
			l.exec(`INSERT INTO pride.billing_plans(id,club_id,name,amount_minor,visits,days) VALUES($1,$2,$3,$4,$5,28)`, id("plan", c*3+p), cid, plan.name, plan.amount, plan.visits)
		}
		for g := 0; g < 10; g++ {
			gi := c*10 + g
			gid := id("group", gi)
			trainer := id("trainer", c*5+g/2)
			labels := []string{"Первые шаги · 6–9 лет", "Основы дзюдо · 10–13 лет", "Юниоры · 12–15 лет", "Спортивная группа · 14–17 лет"}
			label := labels[g%2]
			if g/2 == 2 || g/2 == 3 {
				label = labels[2+g%2]
			}
			l.exec(`INSERT INTO pride.training_groups(id,club_id,name) VALUES($1,$2,$3)`, gid, cid, label+" · "+strings.Fields(staff(c*5+g/2, "trainer").name)[0])
			child := 0
			for f := 0; f < 15; f++ {
				fi := gi*15 + f
				fid := id("family", fi)
				father := adult(fi, false, "parent")
				mother := adult(fi, true, "parent")
				phone := fmt.Sprintf("+375 29 000 %02d %02d", fi/100, fi%100)
				email := strings.TrimPrefix(mother.login, "parent.") + "@example.com"
				l.exec(`INSERT INTO pride.families(id,club_id,name,contact_name,phone,email) VALUES($1,$2,$3,$4,$5,$6)`, fid, cid, familyName(fi), mother.name, phone, email)
				for pi, p := range []person{father, mother} {
					uid := id("parent", fi*2+pi)
					l.user(uid, p, hash, false)
					l.exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, fid, uid)
				}
				count := 1
				if f < 5 {
					count = 2
				}
				for sibling := 0; sibling < count; sibling++ {
					ci := gi*20 + child
					mid := id("member", ci)
					minAge := 6
					if g%2 == 1 {
						minAge = 10
					}
					if g/2 == 2 || g/2 == 3 {
						minAge = 12 + (g%2)*2
					}
					birth := anchor.AddDate(-(minAge + (f % 3) + sibling), -(fi % 10), -(fi % 20))
					l.exec(`INSERT INTO pride.participants(id,full_name,birth_date) VALUES($1,$2,$3::date)`, id("child", ci), childName(fi, sibling), day(birth))
					l.exec(`INSERT INTO pride.club_memberships(id,participant_id,club_id,joined_on,contact_kind,contact_value,contact_name) VALUES($1,$2,$3,$4::date,'phone',$5,$6)`, mid, id("child", ci), cid, day(from), phone, mother.name)
					l.exec(`INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3)`, fid, cid, mid)
					l.exec(`INSERT INTO pride.group_memberships(id,club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,$4,$5::date)`, id("group_member", ci), cid, gid, mid, day(from))
					l.billing(ci, c, anchor)
					child++
				}
			}
			l.schedule(gi, g, c, trainer, from, until, anchor, &result)
		}
		l.exec(`INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'club',$1,'created',$3)`, cid, id("owner", 0), json.RawMessage(`{"source":"demo_dataset","groups":10,"children":200}`))
	}
	// Fresh databases have no planner statistics after a bulk load. Populate them
	// before the first family/profile request (which joins historical sessions).
	l.exec(`ANALYZE pride.clubs, pride.staff_users, pride.staff_club_roles,
 pride.participants, pride.club_memberships, pride.families, pride.family_children,
 pride.family_access, pride.training_groups, pride.group_memberships,
 pride.schedule_rules, pride.training_sessions, pride.attendance_records,
 pride.billing_plans, pride.payments, pride.subscriptions`)
	if l.err != nil {
		return result, l.err
	}
	raw, err = json.Marshal(result)
	if err != nil {
		return result, err
	}
	l.exec(`INSERT INTO pride.audit_events(id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'demo_dataset',$1,'created',$3)`, id("marker", 0), id("owner", 0), raw)
	if l.err != nil {
		return result, l.err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
func (l *loader) user(uid string, p person, hash string, owner bool) {
	l.exec(`INSERT INTO pride.staff_users(id,login,display_name,password_hash,is_owner) VALUES($1,$2,$3,$4,$5)`, uid, p.login, p.name, hash, owner)
}
