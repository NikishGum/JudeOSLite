package training_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/training/attendance"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/jackc/pgx/v5/pgconn"
)

func (f *fixture) rejectAudit(t *testing.T, action, entity string) {
	f.exec(t, `CREATE OR REPLACE FUNCTION pride.test_reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.action=TG_ARGV[0] AND (TG_ARGV[1]='' OR NEW.entity_id::text=TG_ARGV[1]) THEN RAISE EXCEPTION 'synthetic late transaction failure'; END IF; RETURN NEW; END $$`)
	f.exec(t, fmt.Sprintf(`CREATE TRIGGER test_reject_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.test_reject_audit('%s','%s')`, action, entity))
}
func (f *fixture) allowAudit(t *testing.T) {
	f.exec(t, `DROP TRIGGER test_reject_audit ON pride.audit_events`)
}
func TestTransactionFailureIsAtomic(t *testing.T) {
	f := setup(t)
	trainer := f.users["trainer"]
	for i := 0; i < 3; i++ {
		f.member(t, f.a, fmt.Sprintf("Member %d", i), "1999-01-01", nil)
	}
	id := f.session(t, f.a, trainer.id, f.a.group, "2000-02-01")
	path := trainerPath(f.a, id)
	// Failure after all snapshot inserts must also undo the freeze and open audit.
	f.rejectAudit(t, "journal_opened", id)
	stable := f.fingerprint(t)
	status(t, trainer.call(t, "POST", path+"/open", version(1)), 503)
	if f.fingerprint(t) != stable || f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE training_session_id=$1`, id) != 0 {
		t.Fatal("partial snapshot after open failure")
	}
	f.allowAudit(t)
	j := f.open(t, f.a, id)
	f.rejectAudit(t, "marked", j.Records[0].ID)
	stable = f.fingerprint(t)
	status(t, trainer.call(t, "PUT", path+"/records/"+j.Records[0].ID, mark(j, 0, "present", nil)), 503)
	if f.fingerprint(t) != stable {
		t.Fatal("partial mark/version/audit")
	}
	f.allowAudit(t)
	// Fail midway through defaults, then after all defaults at the final close audit.
	for _, failure := range []struct{ action, entity string }{{"marked", j.Records[1].ID}, {"journal_closed", id}} {
		f.rejectAudit(t, failure.action, failure.entity)
		stable = f.fingerprint(t)
		status(t, trainer.call(t, "POST", path+"/close", version(j.Session.Version)), 503)
		if f.fingerprint(t) != stable || f.count(t, `SELECT count(*) FROM pride.attendance_records WHERE training_session_id=$1 AND state IS NOT NULL`, id) != 0 {
			t.Fatal("partial automatic defaults/closed status/audit")
		}
		f.allowAudit(t)
	}
	j = parse[attendance.Journal](t, trainer.call(t, "POST", path+"/close", version(j.Session.Version)), 200)
	f.rejectAudit(t, "corrected", j.Records[0].ID)
	stable = f.fingerprint(t)
	status(t, f.users["admin"].call(t, "PUT", adminPath(f.a, id)+"/records/"+j.Records[0].ID, map[string]any{"version": j.Records[0].Version, "state": "present", "absenceReason": nil, "changeReason": "Synthetic correction"}), 503)
	if f.fingerprint(t) != stable {
		t.Fatal("partial correction/value/reason/author/audit")
	}
	f.allowAudit(t)
	status(t, f.users["admin"].call(t, "PUT", adminPath(f.a, id)+"/records/"+j.Records[0].ID, map[string]any{"version": j.Records[0].Version, "state": "present", "absenceReason": nil, "changeReason": "Synthetic correction"}), 200)
}
func TestHistoricalSnapshotSurvivesChanges(t *testing.T) {
	f := setup(t)
	a := f.member(t, f.a, "A continuing", "1999-01-01", nil)
	b := f.member(t, f.a, "B archived", "1999-01-01", nil)
	f.member(t, f.a, "Ends on boundary", "1999-01-01", ptr("2000-02-01"))
	f.member(t, f.a, "Starts after boundary", "2000-02-03", nil)
	future := fmt.Sprintf("%d-01-01", time.Now().UTC().Year()+2)
	rule := f.seedRule(t, f.a, schedule.Input{Mode: "date_list", Dates: []string{"2000-02-01", "2000-02-02", future}, Time: "18:00", Duration: 60, From: "2000-02-01"})
	f.generate(t, time.Date(2000, 1, 31, 0, 0, 0, 0, time.UTC))
	ids := []string{}
	before := map[string]string{}
	for _, date := range []string{"2000-02-01", "2000-02-02"} {
		id := f.id(t, `SELECT id::text FROM pride.training_sessions WHERE group_id=$1 AND starts_on=$2::date`, f.a.group, date)
		j := f.open(t, f.a, id)
		if len(j.Records) != 2 || j.Records[0].MembershipID != a || j.Records[1].MembershipID != b {
			t.Fatal("wrong dated snapshot")
		}
		if len(ids) == 0 {
			j = parse[attendance.Journal](t, f.users["trainer"].call(t, "POST", trainerPath(f.a, id)+"/close", version(j.Session.Version)), 200)
		}
		ids = append(ids, id)
		before[id] = f.snapshot(t, id)
	}

	// A genuine dated API version change retires the future plan while retaining
	// both real open/closed journals and their entire persisted session rows.
	rows := map[string]string{}
	for _, id := range ids {
		rows[id] = f.id(t, `SELECT to_jsonb(s)::text FROM pride.training_sessions s WHERE id=$1`, id)
	}
	f.saveRule(t, schedule.Input{GroupID: f.a.group, TrainerID: f.users["trainer"].id, Mode: "date_list", Dates: []string{future}, Time: "20:00", Duration: 60, From: future, Version: 1}, rule)
	f.generate(t, time.Date(2000, 1, 31, 0, 0, 0, 0, time.UTC))
	for id, row := range rows {
		if f.id(t, `SELECT to_jsonb(s)::text FROM pride.training_sessions s WHERE id=$1`, id) != row {
			t.Fatal("version/regeneration rewrote historical journal")
		}
	}
	target := f.group(t, f.a.club)
	interval := f.id(t, `SELECT id::text FROM pride.group_memberships WHERE club_membership_id=$1`, a)
	// Retrospective transfer alters today's source query, never already frozen rows.
	status(t, f.users["admin"].call(t, "POST", f.a.base()+"/participants/"+a+"/groups/"+interval+"/transfer", map[string]any{"version": 1, "date": "2000-01-15", "groupId": target}), 200)
	f.exec(t, `UPDATE pride.club_memberships SET archived_at=now(),version=version+1 WHERE id=$1`, b)
	parse[groups.Group](t, f.users["admin"].call(t, "POST", f.a.base()+"/groups/"+f.a.group+"/archive", version(1)), 200)
	stable := f.fingerprint(t)
	// Archive skips extension; even a historical regeneration must not touch either journal.
	f.generate(t, time.Date(2000, 1, 31, 0, 0, 0, 0, time.UTC))
	for _, id := range ids {
		j := parse[attendance.Journal](t, f.users["trainer"].call(t, "POST", trainerPath(f.a, id)+"/open", version(1)), 200)
		if f.snapshot(t, id) != before[id] || len(j.Records) != 2 || !j.Records[1].Archived {
			t.Fatal("transfer/archive/reopen rebuilt snapshot")
		}
	}
	if f.fingerprint(t) != stable {
		t.Fatal("regeneration/reopen rewrote journals/audit")
	}
	j := f.journal(t, f.a, ids[1])
	parse[attendance.Journal](t, f.users["trainer"].call(t, "POST", trainerPath(f.a, ids[1])+"/close", version(j.Session.Version)), 200)
	if f.snapshot(t, ids[1]) != before[ids[1]] {
		t.Fatal("closing archived group rebuilt snapshot")
	}
}
func TestCrossClubAndSnapshotForeignKeys(t *testing.T) {
	f := setup(t)
	a := f.member(t, f.a, "A member", "1999-01-01", nil)
	a2 := f.member(t, f.a, "A second", "1999-01-01", nil)
	b := f.member(t, f.b, "B member", "1999-01-01", nil)
	ia := f.id(t, `SELECT id::text FROM pride.group_memberships WHERE club_membership_id=$1`, a)
	ib := f.id(t, `SELECT id::text FROM pride.group_memberships WHERE club_membership_id=$1`, b)
	ruleB := f.id(t, `INSERT INTO pride.schedule_rules(club_id,group_id,trainer_id,mode,weekdays,starts_time,duration_minutes,timezone,valid_from) VALUES($1,$2,$3,'weekly',ARRAY[1],'18:00',60,'Europe/Minsk','2000-01-01') RETURNING id::text`, f.b.club, f.b.group, f.users["foreign"].id)
	id := f.session(t, f.a, f.users["trainer"].id, f.a.group, "2000-02-01")
	wrongGroup := f.group(t, f.a.club)
	otherSession := f.session(t, f.a, f.users["trainer"].id, wrongGroup, "2000-02-01")
	cases := []struct {
		name, q string
		args    []any
	}{
		{"rule group", `INSERT INTO pride.schedule_rules(club_id,group_id,trainer_id,mode,weekdays,starts_time,duration_minutes,timezone,valid_from) VALUES($1,$2,$3,'weekly',ARRAY[1],'18:00',60,'Europe/Minsk','2000-01-01')`, []any{f.a.club, f.b.group, f.users["trainer"].id}},
		{"session group", `INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,'2000-03-01','Europe/Minsk','2000-03-01 15:00Z','2000-03-01 16:00Z')`, []any{f.a.club, f.b.group, f.users["trainer"].id}},
		{"session rule", `INSERT INTO pride.training_sessions(club_id,group_id,schedule_rule_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,$4,'2000-03-01','Europe/Minsk','2000-03-01 15:00Z','2000-03-01 16:00Z')`, []any{f.a.club, f.a.group, ruleB, f.users["trainer"].id}},
		{"record card club", `INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES($1,$2,$3,$4,$5)`, []any{f.a.club, id, f.a.group, b, ib}},
		{"record source group", `INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES($1,$2,$3,$4,$5)`, []any{f.a.club, otherSession, wrongGroup, a, ia}},
		{"record source member", `INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES($1,$2,$3,$4,$5)`, []any{f.a.club, id, f.a.group, a2, ia}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := f.fingerprint(t)
			_, err := f.pool.Exec(context.Background(), tc.q, tc.args...)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "23503" {
				t.Fatalf("expected FK violation, got %v", err)
			}
			if f.fingerprint(t) != before {
				t.Fatal("invalid relationship partially persisted")
			}
		})
	}
	// Public API also validates current trainer club (historical staff FK alone cannot).
	future := time.Now().AddDate(2, 0, 0).Format("2006-01-02")
	for _, in := range []schedule.Input{
		{GroupID: f.b.group, TrainerID: f.users["trainer"].id, Mode: "once", Dates: []string{future}, From: future, Time: "18:00", Duration: 60},
		{GroupID: f.a.group, TrainerID: f.users["foreign"].id, Mode: "once", Dates: []string{future}, From: future, Time: "18:00", Duration: 60},
	} {
		before := f.fingerprint(t)
		status(t, f.users["admin"].call(t, "POST", f.a.base()+"/schedule/preview", in), 404)
		if f.fingerprint(t) != before {
			t.Fatal("invalid API relationship mutated data")
		}
	}
}
