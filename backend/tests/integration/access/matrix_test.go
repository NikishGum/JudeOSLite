package access_test

import (
	"fmt"
	"testing"

	"github.com/NikishGum/JudeOSLite/backend/internal/audit"
	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
)

func TestAccessReadMatrix(t *testing.T) {
	f := setup(t)
	// Expected scope is explicit and independent of server permission helpers.
	type role struct {
		name                               string
		adminA, adminB, trainerA, trainerB bool
	}
	roles := []role{
		{"owner", true, true, false, false}, {"admin-a", true, false, false, false},
		{"admin-b", false, true, false, false}, {"admin-both", true, true, false, false},
		{"trainer-a", false, false, true, false}, {"trainer-b", false, false, false, true},
		{"trainer-both", false, false, true, true}, {"mixed", true, false, false, true},
		{"replacement", false, false, true, false}, {"unassigned", false, false, false, false},
		{"anonymous", false, false, false, false},
	}
	for _, role := range roles {
		t.Run(role.name, func(t *testing.T) {
			c := f.users[role.name]
			owner := role.name == "owner"
			for i, b := range []branch{f.a, f.b} {
				t.Run(fmt.Sprintf("branch-%d", i), func(t *testing.T) {
					admin, trainer := role.adminA, role.trainerA
					if i == 1 {
						admin, trainer = role.adminB, role.trainerB
					}
					for _, tc := range []struct {
						path    string
						allowed bool
					}{
						{b.base(), admin || trainer}, {"/api/auth/clubs/" + b.club, admin || trainer},
						{b.base() + "/participants?state=all", admin},
						{b.base() + "/participants/" + b.member, admin},
						{b.base() + "/participants/" + b.member + "/groups", admin},
						{b.base() + "/groups", admin},
						{b.base() + "/groups/" + b.group + "/roster?date=2000-02-02", admin},
						{b.base() + "/trainers", admin}, {b.base() + "/trainers?available=true", admin},
						{b.base() + "/schedule", admin || trainer},
						{b.base() + "/schedule/sessions" + period, admin || trainer},
						{b.base() + "/sessions" + period, admin || trainer},
						{b.base() + "/sessions/groups", admin || trainer},
						{b.base() + "/journals?date=2000-02-02", admin},
						{adminPath(b, b.open.Session.ID), admin},
						{adminPath(b, b.closed.Session.ID) + "/records/" + b.closed.Records[0].ID + "/history", admin},
						{b.base() + "/trainer-sessions?date=2000-02-02", trainer},
						{trainerPath(b, b.open.Session.ID), role.name == []string{"trainer-a", "trainer-b"}[i]},
						{trainerPath(b, b.closed.Session.ID), role.name == []string{"trainer-a", "trainer-b"}[i]},
					} {
						want := 404
						if tc.allowed {
							want = 200
						}
						if role.name == "anonymous" {
							want = 401
						}
						t.Run(tc.path, func(t *testing.T) {
							w := c.call(t, "GET", tc.path, nil)
							status(t, w, want)
							if !tc.allowed {
								excludes(t, w, b.club, b.group, b.member, b.name, b.contact)
							}
							if i == 0 {
								excludes(t, w, f.b.contact)
							} else {
								excludes(t, w, f.a.contact)
							}
						})
					}
				})
			}
			for _, path := range []string{"/api/clubs", "/api/staff", "/api/trainer-accounts", "/api/reports/filters", "/api/reports/attendance" + period} {
				t.Run(path, func(t *testing.T) {
					want := 403
					if owner {
						want = 200
					}
					if role.name == "anonymous" {
						want = 401
					}
					w := c.call(t, "GET", path, nil)
					status(t, w, want)
					excludes(t, w, f.a.contact, f.b.contact, "passwordHash", "password_hash", "token_hash")
				})
			}
			for _, path := range []string{"/api/audit/filters", "/api/audit/events"} {
				want := 403
				if role.adminA || role.adminB {
					want = 200
				}
				if role.name == "anonymous" {
					want = 401
				}
				w := c.call(t, "GET", path, nil)
				status(t, w, want)
				excludes(t, w, f.a.contact, f.b.contact)
				if !role.adminA {
					excludes(t, w, f.a.club, f.a.name)
				}
				if !role.adminB {
					excludes(t, w, f.b.club, f.b.name)
				}
			}
		})
	}
}

func TestAccessListsSearchCountsAndPrivacy(t *testing.T) {
	f := setup(t)
	admin, trainer := f.users["admin-a"], f.users["trainer-a"]
	// More than one page of foreign cards must not affect the local hasMore flag.
	f.exec(t, `WITH people AS (INSERT INTO pride.participants(full_name,birth_date) SELECT 'foreign-page-'||n,'1990-01-01'::date FROM generate_series(1,55) n RETURNING id)
		INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) SELECT id,$1,'2000-01-01','email',$2 FROM people`, f.b.club, f.b.contact)
	// Registry counts and search must remain scoped, even for known foreign values.
	for _, q := range []string{"", f.b.contact, "participant-" + f.b.name} {
		w := admin.call(t, "GET", f.a.base()+"/participants?state=all&q="+q, nil)
		v := parse[struct {
			Items []struct {
				ID string `json:"id"`
			}
			HasMore bool `json:"hasMore"`
		}](t, w, 200)
		want := 0
		if q == "" {
			want = 1
		}
		if v.HasMore || len(v.Items) != want {
			t.Fatalf("search/count escaped scope: %s", w.Body.String())
		}
		excludes(t, w, f.b.club, f.b.member, f.b.contact, f.b.name)
	}
	me := parse[struct {
		Clubs []auth.Club `json:"clubs"`
	}](t, admin.call(t, "GET", "/api/auth/me", nil), 200)
	if len(me.Clubs) != 1 || me.Clubs[0].ID != f.a.club {
		t.Fatal("foreign club discovery")
	}
	for _, c := range []client{f.users["owner"], f.users["admin-both"], f.users["trainer-both"], f.users["mixed"]} {
		me := parse[struct {
			Clubs []auth.Club `json:"clubs"`
		}](t, c.call(t, "GET", "/api/auth/me", nil), 200)
		if len(me.Clubs) != 2 {
			t.Fatal("multiple assignments lost")
		}
	}
	for _, tc := range []struct {
		path string
		ids  []string
	}{
		{f.a.base() + "/sessions" + period, []string{f.a.closed.Session.ID, f.a.open.Session.ID}},
		{f.a.base() + "/schedule/sessions" + period, []string{f.a.closed.Session.ID, f.a.open.Session.ID}},
		{f.a.base() + "/sessions/groups", []string{f.a.group}},
		{f.a.base() + "/schedule", []string{f.a.rule}},
	} {
		w := trainer.call(t, "GET", tc.path, nil)
		items := parse[[]struct {
			ID string `json:"id"`
		}](t, w, 200)
		if len(items) != len(tc.ids) {
			t.Fatalf("unexpected scoped list: %s", w.Body.String())
		}
		set := map[string]bool{}
		for _, id := range tc.ids {
			set[id] = true
		}
		for _, item := range items {
			if !set[item.ID] {
				t.Fatalf("foreign item %s", item.ID)
			}
		}
		excludes(t, w, f.b.club, f.b.group, f.b.contact, f.privateGroup, f.privateRule, f.privateSession, f.multiSessionA)
	}
	status(t, trainer.call(t, "GET", f.a.base()+"/sessions"+period+"&groupId="+f.privateGroup, nil), 404)
	status(t, trainer.call(t, "GET", f.a.base()+"/sessions"+period+"&groupId="+f.b.group, nil), 404)
	// A club role alone does not authorize the journal of another actual trainer.
	for _, tc := range []struct {
		b  branch
		id string
	}{{f.a, f.multiSessionA}, {f.b, f.multiSessionB}} {
		status(t, f.users["trainer-both"].call(t, "GET", trainerPath(tc.b, tc.id), nil), 200)
		status(t, f.users["trainer-both"].call(t, "GET", trainerPath(tc.b, tc.b.open.Session.ID), nil), 404)
	}
	for _, path := range []string{trainerPath(f.a, f.a.open.Session.ID), adminPath(f.a, f.a.open.Session.ID), adminPath(f.a, f.a.closed.Session.ID) + "/records/" + f.a.closed.Records[0].ID + "/history", "/api/reports/attendance" + period, "/api/audit/events"} {
		c := f.users["owner"]
		if path == trainerPath(f.a, f.a.open.Session.ID) {
			c = trainer
		}
		w := c.call(t, "GET", path, nil)
		status(t, w, 200)
		excludes(t, w, f.a.contact, f.b.contact, "birthDate", "contactValue", "password_hash", "token_hash", "csrf_token_hash", "before_state", "after_state")
	}
	// Every page, action filter and cursor is checked, not only the first page.
	for i := 0; i < 60; i++ {
		f.exec(t, `INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'training_group',$3,'foreign_marker',$4)`, f.users["owner"].id, f.b.club, f.b.group, `{"contact":"`+f.b.contact+`"}`)
	}
	for i := 0; i < 55; i++ {
		f.exec(t, `INSERT INTO pride.audit_events(actor_id,club_id,entity_type,entity_id,action) VALUES($1,$2,'training_group',$3,'local_marker')`, f.users["owner"].id, f.a.club, f.a.group)
	}
	cursor := ""
	count := 0
	for page := 0; page < 4; page++ {
		w := admin.call(t, "GET", "/api/audit/events?action=local_marker&cursor="+cursor, nil)
		v := parse[audit.Page](t, w, 200)
		excludes(t, w, f.b.club, f.b.name, f.b.contact, "foreign_marker")
		for _, item := range v.Items {
			if item.ClubID == nil || *item.ClubID != f.a.club {
				t.Fatal("foreign audit event")
			}
		}
		count += len(v.Items)
		cursor = v.NextCursor
		if cursor == "" {
			break
		}
	}
	if count != 55 || cursor != "" {
		t.Fatalf("scoped pagination count=%d", count)
	}
	w := admin.call(t, "GET", "/api/audit/events?action=foreign_marker", nil)
	if len(parse[audit.Page](t, w, 200).Items) != 0 {
		t.Fatal("foreign action search leaked events")
	}
	excludes(t, admin.call(t, "GET", "/api/audit/filters", nil), "foreign_marker", f.b.club, f.b.name)
	status(t, admin.call(t, "GET", "/api/audit/events?clubId="+f.b.club, nil), 404)
	status(t, admin.call(t, "GET", "/api/audit/events?clubId=network", nil), 404)
}

func TestAccessForeignIDsAreIndistinguishable(t *testing.T) {
	f := setup(t)
	admin := f.users["admin-a"]
	for _, tc := range []struct{ foreign, absent string }{
		{f.a.base() + "/participants/" + f.b.member, f.a.base() + "/participants/" + missingID},
		{f.a.base() + "/groups/" + f.b.group + "/roster?date=2000-02-02", f.a.base() + "/groups/" + missingID + "/roster?date=2000-02-02"},
		{adminPath(f.a, f.b.open.Session.ID), adminPath(f.a, missingID)},
		{adminPath(f.a, f.a.closed.Session.ID) + "/records/" + f.b.closed.Records[0].ID + "/history", adminPath(f.a, f.a.closed.Session.ID) + "/records/" + missingID + "/history"},
		{f.a.base() + "/sessions" + period + "&groupId=" + f.b.group, f.a.base() + "/sessions" + period + "&groupId=" + missingID},
		{f.a.base() + "/journals?date=2000-02-02&groupId=" + f.b.group, f.a.base() + "/journals?date=2000-02-02&groupId=" + missingID},
	} {
		t.Run(tc.foreign, func(t *testing.T) {
			foreign := admin.call(t, "GET", tc.foreign, nil)
			absent := admin.call(t, "GET", tc.absent, nil)
			status(t, foreign, 404)
			status(t, absent, 404)
			if foreign.Body.String() != absent.Body.String() {
				t.Fatal("foreign ID existence disclosed")
			}
		})
	}
	for _, id := range []string{f.a.open.Session.ID, f.b.open.Session.ID, missingID} {
		w := f.users["replacement"].call(t, "GET", trainerPath(f.a, id), nil)
		status(t, w, 404)
		excludes(t, w, id, f.a.name, f.b.name)
	}
}
