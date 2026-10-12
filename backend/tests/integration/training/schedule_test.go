package training_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/training/schedule"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
)

// Only seed immutable historical inputs with SQL; invoke the production generator
// with explicit clock values. Expected civil dates below are hand enumerated.
func (f *fixture) seedRule(t *testing.T, b branch, in schedule.Input) string {
	return f.id(t, `INSERT INTO pride.schedule_rules(club_id,group_id,trainer_id,mode,weekdays,day_parity,selected_dates,starts_time,duration_minutes,timezone,valid_from,valid_until)
 VALUES($1,$2,$3,$4,$5::smallint[],nullif($6,''),$7::date[],$8::time,$9,'Europe/Minsk',$10::date,$11::date) RETURNING id::text`, b.club, b.group, f.users["trainer"].id, in.Mode, in.Weekdays, in.Parity, in.Dates, in.Time, in.Duration, in.From, in.Until)
}
func (f *fixture) generate(t *testing.T, now time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := f.generator.Generate(ctx, now); err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) dates(t *testing.T, group string) string {
	return f.id(t, `SELECT coalesce(string_agg(to_char(starts_on,'YYYY-MM-DD'),',' ORDER BY starts_on),'') FROM pride.training_sessions WHERE group_id=$1 AND superseded_at IS NULL`, group)
}
func TestGeneratorCivilDates(t *testing.T) {
	f := setup(t)
	cases := []struct {
		name, mode, parity, from, until, now, want string
		weekdays                                   []int
		dates                                      []string
	}{
		{name: "leap even", mode: "day_parity", parity: "even", from: "2000-02-27", until: "2000-03-05", now: "2000-02-27T12:00:00Z", want: "2000-02-28,2000-03-02,2000-03-04"},
		{name: "leap odd reset", mode: "day_parity", parity: "odd", from: "2000-02-27", until: "2000-03-04", now: "2000-02-27T12:00:00Z", want: "2000-02-27,2000-02-29,2000-03-01,2000-03-03"},
		{name: "nonleap century", mode: "day_parity", parity: "odd", from: "1900-02-27", until: "1900-03-04", now: "1900-02-27T00:00:00Z", want: "1900-02-27,1900-03-01,1900-03-03"},
		{name: "thirty day month", mode: "day_parity", parity: "even", from: "2000-04-28", until: "2000-05-03", now: "2000-04-28T12:00:00Z", want: "2000-04-28,2000-04-30,2000-05-02"},
		{name: "thirty one day month", mode: "day_parity", parity: "odd", from: "2000-01-29", until: "2000-02-04", now: "2000-01-29T12:00:00Z", want: "2000-01-29,2000-01-31,2000-02-01,2000-02-03"},
		{name: "weekly year boundary", mode: "weekly", weekdays: []int{1, 6, 7}, from: "2000-12-30", until: "2001-01-03", now: "2000-12-30T12:00:00Z", want: "2000-12-30,2000-12-31,2001-01-01"},
		{name: "local new year horizon", mode: "day_parity", parity: "odd", from: "2001-01-01", until: "2001-01-04", now: "2000-12-31T21:30:00Z", want: "2001-01-01,2001-01-03"},
		{name: "selected far leap dates", mode: "date_list", from: "2000-02-28", dates: []string{"2000-02-28", "2000-02-29", "2040-02-29", "2040-12-31"}, now: "2000-02-27T12:00:00Z", want: "2000-02-28,2000-02-29,2040-02-29,2040-12-31"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := branch{club: f.a.club, group: f.group(t, f.a.club)}
			var until *string
			if tc.until != "" {
				until = ptr(tc.until)
			}
			in := schedule.Input{Mode: tc.mode, Parity: tc.parity, Weekdays: tc.weekdays, Dates: tc.dates, Time: "18:00", Duration: 90, From: tc.from, Until: until}
			f.seedRule(t, b, in)
			now, err := time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			f.concurrentGenerate(t, now)
			if got := f.dates(t, b.group); got != tc.want {
				t.Fatalf("dates %s, want %s", got, tc.want)
			}
			if f.count(t, `SELECT count(*) FROM pride.training_sessions WHERE group_id=$1 AND (ends_at-starts_at<>interval '90 minutes' OR (starts_at AT TIME ZONE timezone)::date<>starts_on OR timezone<>'Europe/Minsk')`, b.group) != 0 {
				t.Fatal("wrong persisted duration/zone/date")
			}
			stable := f.fingerprint(t)
			for range 2 {
				f.generate(t, now)
			}
			f.concurrentGenerate(t, now)
			if f.fingerprint(t) != stable {
				t.Fatal("repeat/concurrent generator rewrote rows/audit")
			}
		})
	}
}
func TestGeneratorHorizonAndNoBackfill(t *testing.T) {
	f := setup(t)
	f.seedRule(t, f.a, schedule.Input{Mode: "day_parity", Parity: "even", Time: "18:00", Duration: 60, From: "2000-01-01"})
	f.generate(t, time.Date(2000, 1, 31, 12, 0, 0, 0, time.UTC))
	if f.count(t, `SELECT count(*) FROM pride.training_sessions`) != 14 || f.count(t, `SELECT count(*) FROM pride.training_sessions WHERE starts_on<'2000-02-01' OR starts_on>='2000-03-01'`) != 0 {
		t.Fatal("backfill or wrong two-month horizon")
	}
	f.generate(t, time.Date(2000, 2, 1, 0, 0, 0, 0, time.UTC))
	if f.count(t, `SELECT count(*) FROM pride.training_sessions`) != 29 || f.count(t, `SELECT count(*) FROM pride.training_sessions WHERE starts_on>='2000-03-01' AND starts_on<'2000-04-01'`) != 15 {
		t.Fatal("rollover lost/duplicated dates")
	}
	stable := f.fingerprint(t)
	f.generate(t, time.Date(2000, 2, 1, 0, 0, 0, 0, time.UTC))
	if stable != f.fingerprint(t) {
		t.Fatal("restart duplicates/rewrites sessions")
	}
}
func (f *fixture) saveRule(t *testing.T, in schedule.Input, old string) (schedule.Rule, schedule.Preview) {
	t.Helper()
	path := f.a.base() + "/schedule"
	method := "POST"
	if old != "" {
		path += "/" + old
		method = "PUT"
	}
	p := parse[schedule.Preview](t, f.users["admin"].call(t, "POST", path+"/preview", in), 200)
	in.PreviewToken = p.Token
	return parse[schedule.Rule](t, f.users["admin"].call(t, method, path, in), 201), p
}
func TestScheduleAPIAndVersionExceptions(t *testing.T) {
	f := setup(t)
	admin := f.users["admin"]
	base := f.a.base() + "/schedule"
	// Relative far dates keep API validation independent of the current month.
	year := time.Now().UTC().Year() + 2
	dates := []string{}
	for day := 1; day <= 4; day++ {
		dates = append(dates, fmt.Sprintf("%d-01-%02d", year, day))
	}
	in := schedule.Input{GroupID: f.a.group, TrainerID: f.users["trainer"].id, Mode: "date_list", Dates: dates, Time: "18:00", Duration: 60, From: dates[0]}
	rule, p := f.saveRule(t, in, "")
	if len(p.Dates) != 4 || f.dates(t, f.a.group) != strings.Join(dates, ",") {
		t.Fatal("far selected dates not created immediately")
	}
	// All four public API modes, including weekly/parity beyond current horizon.
	for _, mode := range []string{"weekly", "day_parity", "once"} {
		input := in
		input.GroupID = f.group(t, f.a.club)
		input.Mode = mode
		input.Dates = nil
		switch mode {
		case "weekly":
			input.Weekdays = []int{1, 3}
		case "day_parity":
			input.Parity = "even"
		case "once":
			input.Dates = []string{dates[0]}
		}
		preview := parse[schedule.Preview](t, admin.call(t, "POST", base+"/preview", input), 200)
		input.PreviewToken = preview.Token
		if mode == "once" {
			v := parse[schedule.Session](t, admin.call(t, "POST", base, input), 201)
			if v.RuleID != nil || len(preview.Dates) != 1 || !v.StartsAt.Equal(preview.Dates[0].StartsAt) {
				t.Fatal("once got rule/wrong preview")
			}
			status(t, admin.call(t, "POST", base, input), 409)
		} else {
			v := parse[schedule.Rule](t, admin.call(t, "POST", base, input), 201)
			if v.Mode != mode || len(preview.Dates) != 0 {
				t.Fatal("wrong mode/horizon")
			}
		}
	}
	ids := []string{}
	for _, date := range dates {
		ids = append(ids, f.id(t, `SELECT id::text FROM pride.training_sessions WHERE group_id=$1 AND starts_on=$2::date AND superseded_at IS NULL`, f.a.group, date))
	}
	cancelled := parse[sessions.Session](t, admin.call(t, "POST", f.a.base()+"/sessions/"+ids[1]+"/cancel", map[string]any{"version": 1, "reason": "Synthetic cancellation"}), 200)
	replacement := parse[sessions.Session](t, admin.call(t, "POST", f.a.base()+"/sessions/"+ids[2]+"/replace-trainer", map[string]any{"version": 1, "trainerId": f.users["replacement"].id}), 200)
	preserved := map[string]string{}
	for _, id := range []string{cancelled.ID, replacement.ID} {
		preserved[id] = f.id(t, `SELECT to_jsonb(s)::text FROM pride.training_sessions s WHERE id=$1`, id)
	}
	change := in
	change.From = dates[1]
	change.Dates = dates[1:]
	change.Time = "20:00"
	change.Version = rule.Version
	next, preview := f.saveRule(t, change, rule.ID)
	if next.ID == rule.ID || next.SeriesID != rule.SeriesID || len(preview.Preserved) != 2 || len(preview.Affected) != 1 || len(preview.Dates) != 1 || preview.Dates[0].Date != dates[3] {
		t.Fatalf("version/exception preview: %+v", preview)
	}
	if f.id(t, `SELECT to_char(valid_until,'YYYY-MM-DD') FROM pride.schedule_rules WHERE id=$1`, rule.ID) != dates[1] || f.count(t, `SELECT count(*) FROM pride.training_sessions WHERE id=$1 AND superseded_at IS NOT NULL`, ids[3]) != 1 {
		t.Fatal("lost historical rule/session ID")
	}
	for range 3 {
		f.generate(t, time.Now())
	}
	for id, row := range preserved {
		if f.id(t, `SELECT to_jsonb(s)::text FROM pride.training_sessions s WHERE id=$1`, id) != row {
			t.Fatal("regeneration erased cancellation/manual replacement")
		}
	}
	if f.dates(t, f.a.group) != strings.Join(dates, ",") {
		t.Fatal("exception resurrected at changed time")
	}
	if f.count(t, `SELECT count(*) FROM pride.training_sessions WHERE id=$1 AND trainer_id=$2 AND is_manual_override`, replacement.ID, f.users["replacement"].id) != 1 {
		t.Fatal("manual trainer lost")
	}
	status(t, admin.call(t, "PUT", base+"/"+rule.ID, change), 409)
	// Reject an audit after retirement and ending the previous version: no partial rule.
	change.From = dates[2]
	change.Dates = dates[2:]
	change.Time = "21:00"
	change.Version = next.Version
	preview = parse[schedule.Preview](t, admin.call(t, "POST", base+"/"+next.ID+"/preview", change), 200)
	change.PreviewToken = preview.Token
	f.rejectAudit(t, "created", "")
	stable := f.fingerprint(t)
	status(t, admin.call(t, "PUT", base+"/"+next.ID, change), 503)
	if f.fingerprint(t) != stable {
		t.Fatal("partial schedule version on audit failure")
	}
	f.allowAudit(t)
}

func (f *fixture) concurrentGenerate(t *testing.T, now time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := make(chan struct{})
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { <-start; errs <- f.generator.Generate(ctx, now) })
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestGeneratorInsertFailureRollback(t *testing.T) {
	f := setup(t)
	f.seedRule(t, f.a, schedule.Input{Mode: "date_list", Dates: []string{"2000-02-01", "2000-02-02", "2000-02-03"}, Time: "18:00", Duration: 60, From: "2000-02-01"})
	f.exec(t, `CREATE FUNCTION pride.test_reject_generation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.starts_on='2000-02-02' THEN RAISE EXCEPTION 'synthetic second insert failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER test_reject_generation BEFORE INSERT ON pride.training_sessions FOR EACH ROW EXECUTE FUNCTION pride.test_reject_generation()`)
	before := f.fingerprint(t)
	now := time.Date(2000, 1, 31, 0, 0, 0, 0, time.UTC)
	if err := f.generator.Generate(context.Background(), now); err == nil {
		t.Fatal("injected generator failure was ignored")
	}
	if f.fingerprint(t) != before {
		t.Fatal("failed per-rule transaction left partial generation")
	}
	f.exec(t, `DROP TRIGGER test_reject_generation ON pride.training_sessions`)
	f.concurrentGenerate(t, now)
	if f.dates(t, f.a.group) != "2000-02-01,2000-02-02,2000-02-03" {
		t.Fatal("retry failed to recover generation")
	}
}
