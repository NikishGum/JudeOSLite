package training_test

import (
	"fmt"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"net/http/httptest"
	"sync"
	"testing"
)

// All callers start together and use independent real PostgreSQL transactions.
// Preserve command indices to compare persisted rows with the winning command.
func race(calls ...func() *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
	start := make(chan struct{})
	out := make([]*httptest.ResponseRecorder, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Go(func() { <-start; out[i] = call() })
	}
	close(start)
	wg.Wait()
	return out
}
func oneWinner(t *testing.T, out []*httptest.ResponseRecorder, losers ...int) int {
	t.Helper()
	winner := -1
	for i, w := range out {
		if w.Code == 200 {
			if winner != -1 {
				t.Fatal("both writes succeeded")
			}
			winner = i
			continue
		}
		ok := false
		for _, code := range losers {
			if w.Code == code {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("unexpected race result %d: %s", w.Code, w.Body.String())
		}
	}
	if winner == -1 {
		t.Fatal("neither write succeeded")
	}
	return winner
}
func (f *fixture) auditCount(t *testing.T, id, action string) int {
	return f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_id=$1 AND action=$2`, id, action)
}
func TestJournalRaces(t *testing.T) {
	f := setup(t)
	trainer := f.users["trainer"]
	admin := f.users["admin"]
	for i := 0; i < 3; i++ {
		f.member(t, f.a, fmt.Sprintf("Member %d", i), "1999-01-01", nil)
	}
	for month, kind := range []string{"open/open", "mark/close", "close/close", "cancel/close", "replacement/mark"} {
		t.Run(kind, func(t *testing.T) {
			for iteration := 0; iteration < 6; iteration++ {
				id := f.session(t, f.a, trainer.id, f.a.group, fmt.Sprintf("2000-%02d-%02d", month+1, iteration+1))
				path := trainerPath(f.a, id)
				if kind == "open/open" {
					for _, w := range race(func() *httptest.ResponseRecorder { return trainer.call(t, "POST", path+"/open", version(1)) }, func() *httptest.ResponseRecorder { return trainer.call(t, "POST", path+"/open", version(1)) }) {
						status(t, w, 200)
					}
					j := f.journal(t, f.a, id)
					if len(j.Records) != 3 || j.Session.Status != "open" || j.Session.Version != 2 || !j.Session.Frozen || f.auditCount(t, id, "journal_opened") != 1 {
						t.Fatal("double/partial snapshot")
					}
					if f.count(t, `SELECT count(DISTINCT club_membership_id) FROM pride.attendance_records WHERE training_session_id=$1`, id) != 3 {
						t.Fatal("duplicate snapshot member")
					}
					continue
				}
				j := f.open(t, f.a, id)
				j = parse[attendance.Journal](t, trainer.call(t, "PUT", path+"/records/"+j.Records[0].ID, mark(j, 0, "absent", ptr("illness"))), 200)
				before := f.snapshot(t, id)
				initial := j.Records[0]
				closeCall := func() *httptest.ResponseRecorder {
					return trainer.call(t, "POST", path+"/close", version(j.Session.Version))
				}
				winner := -1
				switch kind {
				case "mark/close":
					winner = oneWinner(t, race(func() *httptest.ResponseRecorder {
						return trainer.call(t, "PUT", path+"/records/"+j.Records[1].ID, mark(j, 1, "present", nil))
					}, closeCall), 409)
				case "close/close":
					for _, w := range race(closeCall, closeCall) {
						status(t, w, 200)
					}
				case "cancel/close":
					winner = oneWinner(t, race(func() *httptest.ResponseRecorder {
						return admin.call(t, "POST", f.a.base()+"/sessions/"+id+"/cancel", map[string]any{"version": j.Session.Version, "reason": "Synthetic cancellation", "confirmOpen": true})
					}, closeCall), 409)
				case "replacement/mark":
					winner = oneWinner(t, race(func() *httptest.ResponseRecorder {
						return admin.call(t, "POST", f.a.base()+"/sessions/"+id+"/replace-trainer", map[string]any{"version": j.Session.Version, "trainerId": f.users["replacement"].id})
					}, func() *httptest.ResponseRecorder {
						return trainer.call(t, "PUT", path+"/records/"+j.Records[1].ID, mark(j, 1, "present", nil))
					}), 404, 409)
				}
				after := f.journal(t, f.a, id)
				if f.snapshot(t, id) != before {
					t.Fatal("race changed immutable snapshot")
				}
				if after.Records[0].MarkedBy == nil || *after.Records[0].MarkedBy != trainer.id || !after.Records[0].MarkedAt.Equal(*initial.MarkedAt) || after.Records[0].Version != initial.Version {
					t.Fatal("race rewrote existing author/mark")
				}
				if kind == "replacement/mark" {
					if after.Session.Status != "open" || f.auditCount(t, id, "journal_closed") != 0 {
						t.Fatal("replacement closed journal")
					}
					if winner == 0 {
						if after.Session.TrainerID != f.users["replacement"].id || after.Records[1].State != nil || f.auditCount(t, id, "replace-trainer") != 1 {
							t.Fatal("replacement/mark winner inconsistent")
						}
						status(t, trainer.call(t, "GET", path, nil), 404)
					} else {
						if after.Session.TrainerID != trainer.id || after.Records[1].State == nil || *after.Records[1].State != "present" || f.auditCount(t, id, "replace-trainer") != 0 {
							t.Fatal("mark/replacement winner inconsistent")
						}
						changed := parse[sessions.Session](t, admin.call(t, "POST", f.a.base()+"/sessions/"+id+"/replace-trainer", map[string]any{"version": after.Session.Version, "trainerId": f.users["replacement"].id}), 200)
						status(t, trainer.call(t, "GET", path, nil), 404)
						next := parse[attendance.Journal](t, f.users["replacement"].call(t, "GET", path, nil), 200)
						if changed.TrainerID != f.users["replacement"].id || *next.Records[1].MarkedBy != trainer.id {
							t.Fatal("replacement erased previous author")
						}
					}
					continue
				}
				if kind == "cancel/close" && winner == 0 {
					if after.Session.Status != "cancelled" || after.Records[1].State != nil || after.Records[2].State != nil || f.auditCount(t, id, "journal_closed") != 0 || f.auditCount(t, id, "cancel") != 1 {
						t.Fatal("cancellation produced defaults/partial close")
					}
					continue
				}
				if kind == "mark/close" && winner == 0 {
					if after.Session.Status != "open" || *after.Records[1].State != "present" || after.Records[2].State != nil || f.auditCount(t, id, "journal_closed") != 0 {
						t.Fatal("losing close wrote defaults")
					}
					after = parse[attendance.Journal](t, trainer.call(t, "POST", path+"/close", version(after.Session.Version)), 200)
				}
				if after.Session.Status != "closed" || after.Session.ClosedAt == nil || f.auditCount(t, id, "journal_closed") != 1 {
					t.Fatal("ambiguous/duplicate closure")
				}
				for i, r := range after.Records {
					if r.State == nil {
						t.Fatal("closed journal contains unmarked member")
					}
					if i == 2 || (i == 1 && !(kind == "mark/close" && winner == 0)) {
						if *r.State != "absent" || r.Reason == nil || *r.Reason != "unknown" || *r.Origin != "close_default" || r.Version != 2 || f.auditCount(t, r.ID, "marked") != 1 {
							t.Fatal("missing/duplicate default absence")
						}
					}
				}
				stable := f.fingerprint(t)
				status(t, trainer.call(t, "POST", path+"/close", version(j.Session.Version)), 200)
				status(t, trainer.call(t, "POST", path+"/open", version(1)), 200)
				if f.fingerprint(t) != stable {
					t.Fatal("retry changed closed journal/audit")
				}
			}
		})
	}
}
func TestCorrectionRaces(t *testing.T) {
	f := setup(t)
	for i := 0; i < 3; i++ {
		f.member(t, f.a, fmt.Sprintf("Member %d", i), "1999-01-01", nil)
	}
	for iteration := 0; iteration < 6; iteration++ {
		id := f.session(t, f.a, f.users["trainer"].id, f.a.group, fmt.Sprintf("2000-06-%02d", iteration+1))
		j := f.open(t, f.a, id)
		j = parse[attendance.Journal](t, f.users["trainer"].call(t, "POST", trainerPath(f.a, id)+"/close", version(j.Session.Version)), 200)
		snap := f.snapshot(t, id)
		session := f.id(t, `SELECT to_jsonb(s)::text FROM pride.training_sessions s WHERE id=$1`, id)
		path := adminPath(f.a, id) + "/records/" + j.Records[0].ID
		commands := []map[string]any{{"version": j.Records[0].Version, "state": "present", "absenceReason": nil, "changeReason": "Owner checked"}, {"version": j.Records[0].Version, "state": "absent", "absenceReason": "excused", "changeReason": "Admin checked"}}
		winner := oneWinner(t, race(func() *httptest.ResponseRecorder { return f.users["owner"].call(t, "PUT", path, commands[0]) }, func() *httptest.ResponseRecorder { return f.users["admin"].call(t, "PUT", path, commands[1]) }), 409)
		after := f.journal(t, f.a, id)
		r := after.Records[0]
		if r.Version != j.Records[0].Version+1 || *r.State != commands[winner]["state"] || *r.Origin != "correction" || f.auditCount(t, r.ID, "corrected") != 1 {
			t.Fatal("correction overwrite/duplicate audit")
		}
		if f.id(t, `SELECT change_reason FROM pride.attendance_records WHERE id=$1`, r.ID) != commands[winner]["changeReason"] {
			t.Fatal("losing explanation overwrote winner")
		}
		if f.snapshot(t, id) != snap || f.id(t, `SELECT to_jsonb(s)::text FROM pride.training_sessions s WHERE id=$1`, id) != session {
			t.Fatal("correction changed closed session/snapshot")
		}
		stable := f.fingerprint(t)
		status(t, f.users["admin"].call(t, "PUT", path, commands[winner]), 409)
		if stable != f.fingerprint(t) {
			t.Fatal("stale identical correction changed data")
		}
		out := race(func() *httptest.ResponseRecorder {
			return f.users["owner"].call(t, "PUT", adminPath(f.a, id)+"/records/"+after.Records[1].ID, map[string]any{"version": after.Records[1].Version, "state": "present", "absenceReason": nil, "changeReason": "Second member"})
		}, func() *httptest.ResponseRecorder {
			return f.users["admin"].call(t, "PUT", adminPath(f.a, id)+"/records/"+after.Records[2].ID, map[string]any{"version": after.Records[2].Version, "state": "absent", "absenceReason": "illness", "changeReason": "Third member"})
		})
		for _, w := range out {
			status(t, w, 200)
		}
		after = f.journal(t, f.a, id)
		if *after.Records[1].State != "present" || *after.Records[2].Reason != "illness" || f.auditCount(t, after.Records[1].ID, "corrected") != 1 || f.auditCount(t, after.Records[2].ID, "corrected") != 1 {
			t.Fatal("different-record corrections conflicted/lost")
		}
	}
}
