package training_test

import (
	"fmt"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/NikishGum/JudeOSLite/backend/internal/reports"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
)

type expectedMark struct{ state, reason string }
type expectedSession struct {
	id, club, group, date, status string
	marks                         []expectedMark
}

// Independent oracle: only the explicit fixture ledger is input. No report SQL,
// production aggregation helper or current membership/audit query is reused.
func tally(ledger []expectedSession, club, group, from, until string) reports.Totals {
	var out reports.Totals
	for _, s := range ledger {
		if (club != "" && s.club != club) || (group != "" && s.group != group) || s.date < from || s.date > until {
			continue
		}
		switch s.status {
		case "planned":
			out.Planned++
		case "open":
			out.Open++
		case "closed":
			out.Closed++
			for _, m := range s.marks {
				if m.state == "present" {
					out.Present++
					continue
				}
				out.Absent++
				switch m.reason {
				case "illness":
					out.Illness++
				case "excused":
					out.Excused++
				default:
					out.Unknown++
				}
			}
		}
	}
	if n := out.Present + out.Absent; n != 0 {
		p := float64(out.Present) * 100 / float64(n)
		out.Percentage = &p
	}
	return out
}
func equalTotals(a, b reports.Totals) bool {
	ap, bp := a.Percentage, b.Percentage
	a.Percentage = nil
	b.Percentage = nil
	return a == b && ((ap == nil && bp == nil) || (ap != nil && bp != nil && math.Abs(*ap-*bp) < 1e-10))
}
func checkTotals(t *testing.T, got, want reports.Totals) {
	t.Helper()
	if !equalTotals(got, want) {
		t.Fatalf("totals %+v (percentage %v), want %+v (percentage %v)", got, got.Percentage, want, want.Percentage)
	}
}
func (f *fixture) report(t *testing.T, from, until, club, group string) reports.Report {
	q := "?from=" + from + "&until=" + until
	if club != "" {
		q += "&clubId=" + club
	}
	if group != "" {
		q += "&groupId=" + group
	}
	return parse[reports.Report](t, f.users["owner"].call(t, "GET", "/api/reports/attendance"+q, nil), 200)
}
func assertReport(t *testing.T, r reports.Report, ledger []expectedSession, club, group string) {
	t.Helper()
	checkTotals(t, r.Total, tally(ledger, club, group, r.From, r.Until))
	for _, row := range r.Clubs {
		if row.ClubID == nil || row.GroupID != nil {
			t.Fatal("invalid club identity")
		}
		checkTotals(t, row.Totals, tally(ledger, *row.ClubID, group, r.From, r.Until))
	}
	for _, row := range r.Groups {
		if row.GroupID == nil || row.ClubID == nil {
			t.Fatal("invalid group identity")
		}
		checkTotals(t, row.Totals, tally(ledger, *row.ClubID, *row.GroupID, r.From, r.Until))
	}
}
func TestOwnerReportIndependentLedger(t *testing.T) {
	f := setup(t)
	for i := 0; i < 3; i++ {
		f.member(t, f.a, fmt.Sprintf("A member %d", i), "1999-01-01", nil)
	}
	for i := 0; i < 2; i++ {
		f.member(t, f.b, fmt.Sprintf("B member %d", i), "1999-01-01", nil)
	}
	empty := branch{club: f.a.club, group: f.group(t, f.a.club)}
	zeroGroup := f.group(t, f.b.club)
	ledger := []expectedSession{
		{club: f.a.club, group: f.a.group, date: "2000-02-01", status: "closed", marks: []expectedMark{{"present", ""}, {"absent", "illness"}, {"", ""}}},
		{club: f.a.club, group: f.a.group, date: "2000-02-02", status: "closed", marks: []expectedMark{{"present", ""}, {"absent", "excused"}, {"absent", "illness"}}},
		{club: f.b.club, group: f.b.group, date: "2000-02-01", status: "closed", marks: []expectedMark{{"present", ""}, {"present", ""}}},
		{club: empty.club, group: empty.group, date: "2000-02-01", status: "closed"},
		{club: f.a.club, group: f.a.group, date: "2000-02-03", status: "open", marks: []expectedMark{{"present", ""}, {"", ""}, {"", ""}}},
		{club: f.a.club, group: f.a.group, date: "2000-02-04", status: "planned"},
		{club: f.a.club, group: f.a.group, date: "2000-02-05", status: "cancelled", marks: []expectedMark{{"absent", "illness"}, {"", ""}, {"", ""}}},
		{club: f.a.club, group: f.a.group, date: "2000-02-06", status: "retired"},
		{club: f.a.club, group: f.a.group, date: "2000-01-31", status: "closed", marks: []expectedMark{{"", ""}, {"", ""}, {"", ""}}},
		{club: f.a.club, group: f.a.group, date: "2000-03-01", status: "closed", marks: []expectedMark{{"", ""}, {"", ""}, {"", ""}}},
	}
	for i := range ledger {
		s := &ledger[i]
		b := branch{club: s.club, group: s.group}
		c := f.users["trainer"]
		if b.club == f.b.club {
			c = f.users["foreign"]
		}
		if i == 0 {
			// Actual historical midnight differs from both UTC date and original occurrence.
			s.id = f.id(t, `INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at,is_manual_override) VALUES($1,$2,$3,'1999-12-31','Europe/Minsk','2000-02-01 00:30'::timestamp AT TIME ZONE 'Europe/Minsk','2000-02-01 01:30'::timestamp AT TIME ZONE 'Europe/Minsk',true) RETURNING id::text`, b.club, b.group, c.id)
		} else {
			s.id = f.session(t, b, c.id, b.group, s.date)
		}
		if s.status == "planned" {
			continue
		}
		if s.status == "retired" {
			f.exec(t, `UPDATE pride.training_sessions SET superseded_at=now(),version=version+1 WHERE id=$1`, s.id)
			continue
		}
		path := trainerPath(b, s.id)
		j := parse[attendance.Journal](t, c.call(t, "POST", path+"/open", version(1)), 200)
		if len(j.Records) != len(s.marks) {
			t.Fatal("fixture roster size differs from ledger")
		}
		for index, m := range s.marks {
			if m.state != "" {
				var reason *string
				if m.reason != "" {
					reason = ptr(m.reason)
				}
				j = parse[attendance.Journal](t, c.call(t, "PUT", path+"/records/"+j.Records[index].ID, mark(j, index, m.state, reason)), 200)
			}
		}
		switch s.status {
		case "closed":
			status(t, c.call(t, "POST", path+"/close", version(j.Session.Version)), 200)
		case "cancelled":
			status(t, f.users["owner"].call(t, "POST", b.base()+"/sessions/"+s.id+"/cancel", map[string]any{"version": j.Session.Version, "confirmOpen": true, "reason": "Synthetic exclusion"}), 200)
		}
	}
	from, until := "2000-02-01", "2000-02-29"
	percent := 50.0
	manual := reports.Totals{Closed: 4, Planned: 1, Open: 1, Present: 4, Absent: 4, Unknown: 1, Illness: 2, Excused: 1, Percentage: &percent}
	checkTotals(t, tally(ledger, "", "", from, until), manual)
	initial := f.report(t, from, until, "", "")
	assertReport(t, initial, ledger, "", "")
	checkTotals(t, initial.Total, manual)
	if len(initial.Clubs) != 2 || len(initial.Groups) != 4 {
		t.Fatal("missing empty group/branch comparison")
	}
	for _, filter := range []struct{ club, group string }{{f.a.club, ""}, {f.b.club, ""}, {"", f.a.group}, {f.a.club, empty.group}, {f.b.club, zeroGroup}} {
		r := f.report(t, from, until, filter.club, filter.group)
		assertReport(t, r, ledger, filter.club, filter.group)
	}
	for _, period := range [][2]string{{"2000-02-01", "2000-02-01"}, {"2000-01-31", "2000-02-01"}, {"2000-02-29", "2000-03-01"}, {"2000-04-01", "2000-04-30"}} {
		r := f.report(t, period[0], period[1], "", "")
		assertReport(t, r, ledger, "", "")
	}
	// Retrospective transfer/archive must not move historical sums between groups.
	member := f.id(t, `SELECT club_membership_id::text FROM pride.group_memberships WHERE group_id=$1 ORDER BY id LIMIT 1`, f.a.group)
	interval := f.id(t, `SELECT id::text FROM pride.group_memberships WHERE club_membership_id=$1`, member)
	status(t, f.users["admin"].call(t, "POST", f.a.base()+"/participants/"+member+"/groups/"+interval+"/transfer", map[string]any{"version": 1, "date": "2000-01-15", "groupId": empty.group}), 200)
	f.exec(t, `UPDATE pride.club_memberships SET archived_at=now(),version=version+1 WHERE id=$1`, member)
	parse[groups.Group](t, f.users["admin"].call(t, "POST", f.a.base()+"/groups/"+f.a.group+"/archive", version(1)), 200)
	f.exec(t, `UPDATE pride.clubs SET archived_at=now(),version=version+1 WHERE id=$1`, f.b.club)
	after := f.report(t, from, until, "", "")
	assertReport(t, after, ledger, "", "")
	for _, row := range after.Groups {
		if *row.GroupID == f.a.group && !row.GroupArchived {
			t.Fatal("missing historical archive flag")
		}
	}
	for _, row := range after.Clubs {
		if *row.ClubID == f.b.club && !row.ClubArchived {
			t.Fatal("missing archived branch")
		}
	}
	// One real correction: historical audit retains illness; current SQL must use present.
	j := f.journal(t, f.a, ledger[0].id)
	record := j.Records[1]
	status(t, f.users["admin"].call(t, "PUT", adminPath(f.a, j.Session.ID)+"/records/"+record.ID, map[string]any{"version": record.Version, "state": "present", "absenceReason": nil, "changeReason": "Independent control correction"}), 200)
	ledger[0].marks[1] = expectedMark{"present", ""}
	percent = 62.5
	corrected := reports.Totals{Closed: 4, Planned: 1, Open: 1, Present: 5, Absent: 3, Unknown: 1, Illness: 1, Excused: 1, Percentage: &percent}
	checkTotals(t, f.report(t, from, until, "", "").Total, corrected)
	assertReport(t, f.report(t, from, until, "", ""), ledger, "", "")
	if f.count(t, `SELECT count(*) FROM pride.audit_events WHERE entity_id=$1 AND action='corrected' AND before_state->>'absenceReason'='illness' AND after_state->>'state'='present'`, record.ID) != 1 {
		t.Fatal("correction history lost")
	}
	// Concurrent report/correction: every response must be one complete ledger state,
	// and total, club and group levels must all describe that SAME state.
	other := append([]expectedSession(nil), ledger...)
	other[0].marks = append([]expectedMark(nil), ledger[0].marks...)
	other[0].marks[1] = expectedMark{"absent", "illness"}
	for iteration := 0; iteration < 8; iteration++ {
		j = f.journal(t, f.a, j.Session.ID)
		state := "present"
		var reason *string
		if iteration%2 == 0 {
			state = "absent"
			reason = ptr("illness")
		}
		result := race(func() *httptest.ResponseRecorder {
			return f.users["admin"].call(t, "PUT", adminPath(f.a, j.Session.ID)+"/records/"+record.ID, map[string]any{"version": j.Records[1].Version, "state": state, "absenceReason": reason, "changeReason": "Concurrent control"})
		}, func() *httptest.ResponseRecorder {
			return f.users["owner"].call(t, "GET", "/api/reports/attendance?from="+from+"&until="+until, nil)
		})
		status(t, result[0], 200)
		r := parse[reports.Report](t, result[1], 200)
		candidate := ledger
		if !equalTotals(r.Total, tally(ledger, "", "", from, until)) {
			candidate = other
		}
		assertReport(t, r, candidate, "", "")
	}
	// Read-only report must never append audit or mutate the domain rows.
	stable := f.fingerprint(t)
	f.report(t, from, until, "", "")
	if stable != f.fingerprint(t) {
		t.Fatal("report mutated domain data")
	}
}
