package access_test

import (
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/participants"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/corrections"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
)

type command struct {
	method, path string
	body         any
}

func commands(f *fixture, b branch) []command {
	tomorrow := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	card := map[string]any{"version": 1, "joinedOn": "2000-01-01", "contactKind": "email", "contactValue": "updated@example.test", "contactName": "synthetic"}
	in := schedule.Input{GroupID: b.group, TrainerID: b.open.Session.TrainerID, Mode: "weekly", Weekdays: []int{1}, Time: "18:00", Duration: 60, From: tomorrow, Version: 1}
	return []command{
		{"POST", b.base() + "/participants", map[string]any{"identity": map[string]any{"fullName": "synthetic-new", "birthDate": "1990-01-01"}, "joinedOn": "2000-01-01", "contactKind": "email", "contactValue": "new@example.test"}},
		{"PUT", b.base() + "/participants/" + b.member, card},
		{"POST", b.base() + "/participants/" + b.member + "/archive", map[string]int{"version": 1}},
		{"POST", b.base() + "/participants/" + b.member + "/restore", map[string]int{"version": 1}},
		{"POST", b.base() + "/groups", map[string]string{"name": "synthetic-new-group"}},
		{"PUT", b.base() + "/groups/" + b.group, map[string]any{"name": "changed", "version": 1}},
		{"POST", b.base() + "/groups/" + b.group + "/archive", map[string]int{"version": 1}},
		{"POST", b.base() + "/participants/" + b.member + "/groups", map[string]any{"groupId": b.group, "from": "2000-03-01"}},
		{"POST", b.base() + "/participants/" + b.member + "/groups/" + b.interval + "/exclude", map[string]any{"version": 1, "date": "2000-03-01"}},
		{"POST", b.base() + "/participants/" + b.member + "/groups/" + b.interval + "/transfer", map[string]any{"version": 1, "date": "2000-03-01", "groupId": b.group}},
		{"POST", b.base() + "/schedule/preview", in}, {"POST", b.base() + "/schedule", in},
		{"POST", b.base() + "/schedule/" + b.rule + "/preview", in}, {"PUT", b.base() + "/schedule/" + b.rule, in},
		{"POST", b.base() + "/sessions", map[string]any{"groupId": b.group, "trainerId": b.open.Session.TrainerID, "date": tomorrow, "startsTime": "18:00", "durationMinutes": 60}},
		{"POST", b.base() + "/sessions/" + b.open.Session.ID + "/cancel", map[string]any{"version": b.open.Session.Version, "reason": "synthetic", "confirmOpen": true}},
		{"POST", b.base() + "/sessions/" + b.open.Session.ID + "/replace-trainer", map[string]any{"version": b.open.Session.Version, "trainerId": b.open.Session.TrainerID}},
		{"POST", b.base() + "/sessions/" + b.open.Session.ID + "/reschedule", map[string]any{"version": b.open.Session.Version, "date": tomorrow, "startsTime": "18:00", "durationMinutes": 60}},
		{"PUT", adminPath(b, b.closed.Session.ID) + "/records/" + b.closed.Records[0].ID, corrections.Input{Version: b.closed.Records[0].Version, State: "present", ChangeReason: "synthetic correction"}},
		{"POST", b.base() + "/trainers", map[string]string{"login": "new-trainer", "displayName": "synthetic", "password": password}},
		{"PUT", b.base() + "/trainers/" + b.open.Session.TrainerID + "/account", map[string]any{"login": "changed", "displayName": "changed", "disabled": false, "version": 1}},
		{"POST", b.base() + "/trainers/" + b.open.Session.TrainerID + "/password", map[string]any{"version": 1, "password": password}},
		{"POST", b.base() + "/trainers/" + b.open.Session.TrainerID + "/revoke", map[string]int{"version": 1}},
	}
}

func TestAccessDeniedMutationMatrix(t *testing.T) {
	f := setup(t)
	before := f.fingerprint(t)
	// Valid bodies ensure each refusal tests permission rather than malformed input.
	for _, tc := range []struct {
		actor string
		b     branch
	}{
		{"admin-a", f.b}, {"admin-b", f.a}, {"trainer-a", f.a}, {"trainer-b", f.b},
		{"trainer-both", f.a}, {"trainer-both", f.b}, {"mixed", f.b}, {"unassigned", f.a}, {"anonymous", f.a},
	} {
		for _, cmd := range commands(f, tc.b) {
			t.Run(tc.actor+"/"+cmd.method+cmd.path, func(t *testing.T) {
				want := 404
				if tc.actor == "anonymous" {
					want = 401
				}
				w := f.users[tc.actor].call(t, cmd.method, cmd.path, cmd.body)
				status(t, w, want)
				excludes(t, w, f.a.contact, f.b.contact, tc.b.member, tc.b.group)
			})
		}
	}
	for _, actor := range []string{"admin-a", "admin-both", "trainer-a", "mixed", "unassigned", "anonymous"} {
		for _, cmd := range []command{
			{"POST", "/api/clubs", map[string]string{"name": "escalated", "timezone": "UTC"}},
			{"PUT", f.a.base(), map[string]any{"name": "escalated", "timezone": "UTC", "version": 1}},
			{"POST", f.a.base() + "/archive", map[string]int{"version": 1}},
			{"POST", "/api/staff", map[string]any{"login": "escalated", "displayName": "escalated", "password": password, "clubIds": []string{f.a.club}}},
			{"PUT", "/api/staff/" + f.users["trainer-a"].id, map[string]any{"version": 1, "displayName": "escalated", "disabled": false, "clubIds": []string{f.a.club, f.b.club}}},
			{"PUT", "/api/staff/" + f.users["trainer-both"].id + "/account", map[string]any{"version": 1, "login": "changed", "displayName": "changed", "disabled": false}},
			{"POST", "/api/staff/" + f.users["trainer-both"].id + "/password", map[string]any{"version": 1, "password": password}},
			{"POST", "/api/staff/" + f.users["trainer-a"].id + "/trainer-assignments", map[string]any{"clubId": f.b.club}},
		} {
			want := 403
			if actor == "anonymous" {
				want = 401
			}
			status(t, f.users[actor].call(t, cmd.method, cmd.path, cmd.body), want)
		}
	}
	if f.fingerprint(t) != before {
		t.Fatal("denied mutation changed domain rows or audit")
	}
}

func TestAccessSubstitutedIDsAndEscalation(t *testing.T) {
	f := setup(t)
	admin, owner := f.users["admin-a"], f.users["owner"]
	before := f.fingerprint(t)
	// An authorized club in the URL cannot authorize foreign object IDs in URL/body.
	for _, cmd := range []command{
		{"PUT", f.a.base() + "/participants/" + f.b.member, map[string]any{"version": 1, "joinedOn": "2000-01-01", "contactKind": "email", "contactValue": "changed@example.test"}},
		{"POST", f.a.base() + "/participants/" + f.b.member + "/archive", map[string]int{"version": 1}},
		{"PUT", f.a.base() + "/groups/" + f.b.group, map[string]any{"version": 1, "name": "changed"}},
		{"POST", f.a.base() + "/groups/" + f.b.group + "/archive", map[string]int{"version": 1}},
		{"POST", f.a.base() + "/participants/" + f.a.member + "/groups", map[string]any{"groupId": f.b.group, "from": "2000-03-01"}},
		{"POST", f.a.base() + "/participants/" + f.a.member + "/groups/" + f.b.interval + "/exclude", map[string]any{"version": 1, "date": "2000-03-01"}},
		{"POST", f.a.base() + "/participants/" + f.a.member + "/groups/" + f.a.interval + "/transfer", map[string]any{"version": 1, "date": "2000-03-01", "groupId": f.b.group}},
		{"POST", f.a.base() + "/sessions/" + f.b.open.Session.ID + "/cancel", map[string]any{"version": f.b.open.Session.Version, "reason": "synthetic", "confirmOpen": true}},
		{"POST", f.a.base() + "/sessions/" + f.a.open.Session.ID + "/replace-trainer", map[string]any{"version": f.a.open.Session.Version, "trainerId": f.users["trainer-b"].id}},
		{"PUT", adminPath(f.a, f.a.closed.Session.ID) + "/records/" + f.b.closed.Records[0].ID, corrections.Input{Version: 2, State: "present", ChangeReason: "synthetic"}},
		{"PUT", adminPath(f.a, f.b.closed.Session.ID) + "/records/" + f.b.closed.Records[0].ID, corrections.Input{Version: 2, State: "present", ChangeReason: "synthetic"}},
		{"PUT", f.a.base() + "/trainers/" + f.users["trainer-b"].id + "/account", map[string]any{"version": 1, "login": "changed", "displayName": "changed", "disabled": false}},
		{"POST", f.a.base() + "/trainers/" + f.users["trainer-b"].id + "/revoke", map[string]int{"version": 1}},
	} {
		status(t, admin.call(t, cmd.method, cmd.path, cmd.body), 404)
	}
	tomorrow := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	for _, ids := range [][2]string{{f.b.group, f.users["trainer-a"].id}, {f.a.group, f.users["trainer-b"].id}} {
		in := schedule.Input{GroupID: ids[0], TrainerID: ids[1], Mode: "weekly", Weekdays: []int{1}, Time: "18:00", Duration: 60, From: tomorrow, Version: 1}
		status(t, admin.call(t, "POST", f.a.base()+"/schedule/preview", in), 404)
		status(t, admin.call(t, "POST", f.a.base()+"/sessions", map[string]any{"groupId": ids[0], "trainerId": ids[1], "date": tomorrow, "startsTime": "18:00", "durationMinutes": 60}), 404)
	}
	in := schedule.Input{GroupID: f.a.group, TrainerID: f.users["trainer-a"].id, Mode: "weekly", Weekdays: []int{1}, Time: "18:00", Duration: 60, From: tomorrow, Version: 1}
	status(t, admin.call(t, "POST", f.a.base()+"/schedule/"+f.b.rule+"/preview", in), 404)
	// Local administrators cannot change a shared multi-club account, even their own.
	for _, id := range []string{f.users["trainer-both"].id, f.users["mixed"].id} {
		c := admin
		b := f.a
		if id == f.users["mixed"].id {
			c = f.users["admin-b"]
			b = f.b
		}
		status(t, c.call(t, "PUT", b.base()+"/trainers/"+id+"/account", map[string]any{"version": 1, "login": "changed", "displayName": "changed", "disabled": true}), 403)
		status(t, c.call(t, "POST", b.base()+"/trainers/"+id+"/password", map[string]any{"version": 1, "password": password}), 403)
	}
	for _, field := range []string{"isOwner", "role", "clubId", "clubIds", "staffUserId"} {
		body := map[string]any{"login": "escalated", "displayName": "escalated", "password": password, field: "owner"}
		status(t, admin.call(t, "POST", f.a.base()+"/trainers", body), 400)
	}
	for _, field := range []string{"clubId", "participantId", "isOwner", "actorId"} {
		body := map[string]any{"version": 1, "joinedOn": "2000-01-01", "contactKind": "email", "contactValue": "changed@example.test", field: f.b.club}
		status(t, admin.call(t, "PUT", f.a.base()+"/participants/"+f.a.member, body), 400)
	}
	if f.fingerprint(t) != before {
		t.Fatal("substituted ID or escalation changed data")
	}
	// Multi-club identity: local contact is editable, common identity requires owner.
	p := f.id(t, `SELECT participant_id::text FROM pride.club_memberships WHERE id=$1`, f.a.member)
	f.exec(t, `INSERT INTO pride.club_memberships(participant_id,club_id,joined_on,contact_kind,contact_value) VALUES($1,$2,'2000-01-01','email','shared-private@example.test')`, p, f.b.club)
	body := map[string]any{"version": 1, "joinedOn": "2000-01-01", "contactKind": "email", "contactValue": "local-update@example.test", "identity": map[string]any{"version": 1, "fullName": "updated-shared", "birthDate": "1990-01-01"}}
	before = f.fingerprint(t)
	status(t, admin.call(t, "PUT", f.a.base()+"/participants/"+f.a.member, body), 403)
	if f.fingerprint(t) != before {
		t.Fatal("shared identity denial was not atomic")
	}
	delete(body, "identity")
	card := parse[participants.Card](t, admin.call(t, "PUT", f.a.base()+"/participants/"+f.a.member, body), 200)
	if card.ContactValue != "local-update@example.test" {
		t.Fatal("local contact not updated")
	}
	w := admin.call(t, "GET", f.a.base()+"/participants/"+f.a.member, nil)
	excludes(t, w, "shared-private@example.test")
	body["version"] = card.Version
	body["identity"] = map[string]any{"version": 1, "fullName": "updated-shared", "birthDate": "1990-01-01"}
	status(t, owner.call(t, "PUT", f.a.base()+"/participants/"+f.a.member, body), 200)
}

func TestAccessTrainerReplacementClosedJournalAndRecordScope(t *testing.T) {
	f := setup(t)
	trainer, replacement, admin := f.users["trainer-a"], f.users["replacement"], f.users["admin-a"]
	j := f.a.open
	mark := map[string]any{"version": j.Records[0].Version, "sessionVersion": j.Session.Version, "state": "present"}
	before := f.fingerprint(t)
	for _, target := range []string{trainerPath(f.a, j.Session.ID) + "/records/" + f.b.open.Records[0].ID, trainerPath(f.a, f.b.open.Session.ID) + "/records/" + f.b.open.Records[0].ID} {
		status(t, trainer.call(t, "PUT", target, mark), 404)
	}
	for _, c := range []client{replacement, f.users["trainer-both"], admin, f.users["owner"]} {
		status(t, c.call(t, "PUT", trainerPath(f.a, j.Session.ID)+"/records/"+j.Records[0].ID, mark), 404)
		status(t, c.call(t, "POST", trainerPath(f.a, j.Session.ID)+"/close", map[string]int{"version": j.Session.Version}), 404)
	}
	if f.fingerprint(t) != before {
		t.Fatal("foreign record or actual-trainer denial changed data")
	}
	status(t, f.users["trainer-both"].call(t, "POST", trainerPath(f.a, f.multiSessionA)+"/open", map[string]int{"version": 1}), 200)
	before = f.fingerprint(t)
	closed := f.a.closed
	status(t, trainer.call(t, "PUT", trainerPath(f.a, closed.Session.ID)+"/records/"+closed.Records[0].ID, map[string]any{"version": closed.Records[0].Version, "sessionVersion": closed.Session.Version, "state": "present"}), 409)
	status(t, trainer.call(t, "PUT", adminPath(f.a, closed.Session.ID)+"/records/"+closed.Records[0].ID, corrections.Input{Version: closed.Records[0].Version, State: "present", ChangeReason: "synthetic"}), 404)
	if f.fingerprint(t) != before {
		t.Fatal("trainer changed closed journal")
	}
	newSession := parse[sessions.Session](t, admin.call(t, "POST", f.a.base()+"/sessions/"+j.Session.ID+"/replace-trainer", map[string]any{"version": j.Session.Version, "trainerId": replacement.id}), 200)
	before = f.fingerprint(t)
	status(t, trainer.call(t, "GET", trainerPath(f.a, j.Session.ID), nil), 404)
	status(t, trainer.call(t, "PUT", trainerPath(f.a, j.Session.ID)+"/records/"+j.Records[0].ID, mark), 404)
	status(t, trainer.call(t, "POST", trainerPath(f.a, j.Session.ID)+"/close", map[string]int{"version": newSession.Version}), 404)
	if f.fingerprint(t) != before {
		t.Fatal("replaced trainer retained write access")
	}
	w := trainer.call(t, "GET", f.a.base()+"/trainer-sessions?date=2000-02-02", nil)
	if len(parse[[]sessions.Session](t, w, 200)) != 0 {
		t.Fatal("replaced trainer sees session")
	}
	mark["sessionVersion"] = newSession.Version
	marked := parse[attendance.Journal](t, replacement.call(t, "PUT", trainerPath(f.a, j.Session.ID)+"/records/"+j.Records[0].ID, mark), 200)
	if marked.Records[0].MarkedBy == nil || *marked.Records[0].MarkedBy != replacement.id {
		t.Fatal("actual trainer attribution wrong")
	}
	status(t, replacement.call(t, "POST", trainerPath(f.a, j.Session.ID)+"/close", map[string]int{"version": marked.Session.Version}), 200)
	current := parse[corrections.Journal](t, admin.call(t, "GET", adminPath(f.a, j.Session.ID), nil), 200)
	status(t, admin.call(t, "PUT", adminPath(f.a, j.Session.ID)+"/records/"+j.Records[0].ID, corrections.Input{Version: current.Records[0].Version, State: "absent", Reason: ptr("illness"), ChangeReason: "synthetic admin correction"}), 200)
}

func ptr(s string) *string { return &s }

func TestAccessRevocationDisableAndSessionReuse(t *testing.T) {
	f := setup(t)
	admin, trainer := f.users["admin-both"], f.users["trainer-both"]
	// Keep the original cookies: revocation must take effect without logging in again.
	f.exec(t, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id=$2 AND role='administrator'`, admin.id, f.a.club)
	for _, path := range []string{f.a.base() + "/participants", f.a.base() + "/groups", f.a.base() + "/trainers", f.a.base() + "/schedule", f.a.base() + "/sessions" + period, adminPath(f.a, f.a.closed.Session.ID)} {
		status(t, admin.call(t, "GET", path, nil), 404)
	}
	before := f.fingerprint(t)
	for _, cmd := range commands(f, f.a) {
		status(t, admin.call(t, cmd.method, cmd.path, cmd.body), 404)
	}
	if f.fingerprint(t) != before {
		t.Fatal("revoked admin mutated data")
	}
	status(t, admin.call(t, "GET", f.b.base()+"/participants", nil), 200)
	excludes(t, admin.call(t, "GET", "/api/audit/events", nil), f.a.club, f.a.name)
	status(t, admin.call(t, "GET", "/api/audit/events?clubId="+f.a.club, nil), 404)
	me := parse[struct {
		Clubs []auth.Club `json:"clubs"`
	}](t, admin.call(t, "GET", "/api/auth/me", nil), 200)
	if len(me.Clubs) != 1 || me.Clubs[0].ID != f.b.club {
		t.Fatal("revoked club in discovery")
	}
	f.exec(t, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id=$2 AND role='trainer'`, trainer.id, f.a.club)
	status(t, trainer.call(t, "GET", trainerPath(f.a, f.multiSessionA), nil), 404)
	before = f.fingerprint(t)
	status(t, trainer.call(t, "POST", trainerPath(f.a, f.multiSessionA)+"/open", map[string]int{"version": 1}), 404)
	if f.fingerprint(t) != before {
		t.Fatal("revoked trainer opened snapshot")
	}
	status(t, trainer.call(t, "GET", trainerPath(f.b, f.multiSessionB), nil), 200)
	// Removing administrator from a mixed role must leave only trainer capabilities.
	mixed := f.users["mixed"]
	f.exec(t, `INSERT INTO pride.staff_club_roles(staff_user_id,club_id,role) VALUES($1,$2,'trainer')`, mixed.id, f.a.club)
	f.exec(t, `DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1 AND role='administrator'`, mixed.id)
	status(t, mixed.call(t, "GET", f.a.base()+"/participants", nil), 404)
	status(t, mixed.call(t, "GET", f.a.base()+"/sessions"+period, nil), 200)
	status(t, mixed.call(t, "GET", "/api/audit/events", nil), 403)
	// Disable and re-enable do not resurrect any old session of any role.
	for _, name := range []string{"admin-b", "trainer-b", "owner"} {
		c := f.users[name]
		f.exec(t, `UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, c.id)
		for _, path := range []string{"/api/auth/me", f.b.base() + "/participants", trainerPath(f.b, f.b.open.Session.ID), "/api/audit/events", "/api/reports/filters"} {
			status(t, c.call(t, "GET", path, nil), 401)
		}
		status(t, c.call(t, "POST", trainerPath(f.b, f.b.open.Session.ID)+"/close", map[string]int{"version": f.b.open.Session.Version}), 401)
		f.exec(t, `UPDATE pride.staff_users SET disabled_at=NULL WHERE id=$1`, c.id)
		status(t, c.call(t, "GET", "/api/auth/me", nil), 401)
	}
	// Explicit logout revokes the server session even if cookies are replayed.
	status(t, trainer.call(t, "POST", "/api/auth/logout", nil), 200)
	status(t, trainer.call(t, "GET", trainerPath(f.b, f.multiSessionB), nil), 401)
}

func TestAccessAllowedAdministratorWrites(t *testing.T) {
	f := setup(t)
	for _, tc := range []struct {
		actor string
		b     branch
	}{
		{"owner", f.a}, {"owner", f.b}, {"admin-a", f.a}, {"admin-b", f.b},
		{"admin-both", f.a}, {"admin-both", f.b}, {"mixed", f.a},
	} {
		t.Run(tc.actor+"/"+tc.b.name, func(t *testing.T) {
			c, b := f.users[tc.actor], tc.b
			name := tc.actor + "-" + b.name
			trainer := parse[auth.Trainer](t, c.call(t, "POST", b.base()+"/trainers", map[string]string{"login": name, "displayName": name, "password": password}), 201)
			group := parse[groups.Group](t, c.call(t, "POST", b.base()+"/groups", map[string]string{"name": name}), 201)
			card := parse[participants.Card](t, c.call(t, "POST", b.base()+"/participants", map[string]any{
				"identity": map[string]any{"fullName": name, "birthDate": "1990-01-01"},
				"joinedOn": "2000-01-01", "contactKind": "email", "contactValue": name + "@example.test",
				"initialGroup": map[string]any{"groupId": group.ID, "from": "2000-01-01"},
			}), 201)
			if card.ClubID != b.club {
				t.Fatal("new card in wrong club")
			}
			from := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
			until := time.Now().UTC().AddDate(0, 0, 9).Format("2006-01-02")
			in := schedule.Input{GroupID: group.ID, TrainerID: trainer.ID, Mode: "weekly", Weekdays: []int{1}, Time: "18:00", Duration: 60, From: from, Until: &until}
			preview := parse[schedule.Preview](t, c.call(t, "POST", b.base()+"/schedule/preview", in), 200)
			in.PreviewToken = preview.Token
			status(t, c.call(t, "POST", b.base()+"/schedule", in), 201)
			v := parse[sessions.Session](t, c.call(t, "POST", b.base()+"/sessions", map[string]any{"groupId": group.ID, "trainerId": trainer.ID, "date": from, "startsTime": "12:00", "durationMinutes": 60}), 201)
			status(t, c.call(t, "POST", b.base()+"/sessions/"+v.ID+"/cancel", map[string]any{"version": v.Version, "reason": "synthetic cancellation"}), 200)
			// Each admin, including both-club and mixed-role users, may correct their club.
			j := parse[corrections.Journal](t, c.call(t, "GET", adminPath(b, b.closed.Session.ID), nil), 200)
			state := "present"
			var reason *string
			if j.Records[0].State != nil && *j.Records[0].State == "present" {
				state = "absent"
				reason = ptr("unknown")
			}
			status(t, c.call(t, "PUT", adminPath(b, b.closed.Session.ID)+"/records/"+j.Records[0].ID, corrections.Input{Version: j.Records[0].Version, State: state, Reason: reason, ChangeReason: "synthetic permission check"}), 200)
		})
	}
}
