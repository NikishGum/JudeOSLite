package attendance

import (
	"strings"
	"testing"
)

func TestRosterReadingAndFreeze(t *testing.T) {
	f := setup(t)
	f.member(t, "Ended before session", "2027-01-01", ptr("2028-01-03"))
	a := f.member(t, "A Preview", "2028-01-03", nil)
	f.member(t, "Future arrival", "2028-01-04", nil)
	v := f.newSession(t, f.group, "2028-01-03")
	read := func() Journal { return parse[Journal](t, f.trainer.call("GET", f.path(v.ID), nil), 200) }
	before := read()
	for range 3 {
		j := read()
		if j.Roster.Source != "preliminary" || j.Roster.Date != v.Date || j.Roster.Summary != nil || len(j.Roster.Members) != 1 || j.Roster.Members[0].MembershipID != a || len(j.Records) != 0 || j.Session.Frozen || j.Session.Version != 1 {
			t.Fatalf("preview mutated journal or used wrong date: %+v", j)
		}
	}
	if f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE training_session_id=$1`, v.ID) != 0 {
		t.Fatal("GET created records")
	}
	// Change the composition after preview and before open.
	f.exec(t, `DELETE FROM pride.group_memberships WHERE club_membership_id=$1`, a)
	b := f.member(t, "B Arrived before open", "2028-01-03", nil)
	opened := f.open(t, before.Session)
	if opened.Roster.Source != "snapshot" || len(opened.Roster.Members) != 1 || opened.Roster.Members[0].MembershipID != b || opened.Roster.Summary.Unmarked != 1 {
		t.Fatalf("stale preview frozen: %+v", opened)
	}
	f.exec(t, `UPDATE pride.group_memberships SET valid_until='2028-01-04' WHERE club_membership_id=$1`, b)
	f.exec(t, `UPDATE pride.club_memberships SET archived_at=now() WHERE id=$1`, b)
	f.member(t, "Not in snapshot", "2028-01-03", nil)
	marked := f.mark(t, opened, 0, "present", nil)
	if len(marked.Roster.Members) != 1 || !marked.Roster.Members[0].Archived || marked.Roster.Summary.Present != 1 || marked.Roster.Summary.Unmarked != 0 {
		t.Fatal("snapshot or summary rebuilt")
	}
	parse[map[string]any](t, f.owner.call("POST", "/api/clubs/"+f.club+"/sessions/"+v.ID+"/cancel", map[string]any{"version": marked.Session.Version, "reason": "Synthetic cancellation", "confirmOpen": true}), 200)
	if j := read(); j.Roster.Source != "snapshot" || len(j.Roster.Members) != 1 || j.Roster.Summary.Present != 1 {
		t.Fatal("cancelled snapshot lost")
	}
	empty := f.newSession(t, f.emptyGroup, "2028-01-02")
	frozen := f.open(t, empty)
	if frozen.Roster.Source != "snapshot" || frozen.Roster.Summary == nil || len(frozen.Roster.Members) != 0 {
		t.Fatal("empty snapshot not authoritative")
	}
	f.exec(t, `INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2028-01-01')`, f.club, f.emptyGroup, a)
	stillEmpty := parse[Journal](t, f.trainer.call("GET", f.path(empty.ID), nil), 200)
	if len(stillEmpty.Roster.Members) != 0 {
		t.Fatal("empty snapshot replaced by memberships")
	}
}

func TestRosterUsesMovedDateAndUnfrozenCancellation(t *testing.T) {
	f := setup(t)
	f.member(t, "Original day only", "2028-01-01", ptr("2028-01-02"))
	id := f.member(t, "Actual day", "2028-01-02", nil)
	v := f.newSession(t, f.group, "2028-01-01")
	f.exec(t, `UPDATE pride.training_sessions SET starts_at='2028-01-02 18:00+03',ends_at='2028-01-02 19:00+03',is_manual_override=true,version=version+1 WHERE id=$1`, v.ID)
	j := parse[Journal](t, f.trainer.call("GET", f.path(v.ID), nil), 200)
	if j.Roster.Date != "2028-01-02" || len(j.Roster.Members) != 1 || j.Roster.Members[0].MembershipID != id || j.Session.PreviousTrainerName != nil {
		t.Fatal("move mistaken for replacement or roster uses sourceDate")
	}
	parse[map[string]any](t, f.owner.call("POST", "/api/clubs/"+f.club+"/sessions/"+v.ID+"/cancel", map[string]any{"version": j.Session.Version, "reason": "Synthetic cancellation"}), 200)
	j = parse[Journal](t, f.trainer.call("GET", f.path(v.ID), nil), 200)
	if j.Roster.Source != "preliminary" || j.Roster.Summary != nil || j.Session.Frozen || len(j.Records) != 0 {
		t.Fatal("unfrozen cancellation presented as history")
	}
	raw := f.trainer.call("GET", "/api/clubs/"+f.club+"/sessions?from=2028-01-01&until=2028-01-04", nil)
	status(t, raw, 200)
	if strings.Contains(raw.Body.String(), "members") || strings.Contains(raw.Body.String(), "Actual day") {
		t.Fatal("calendar expanded with participant data")
	}
}
