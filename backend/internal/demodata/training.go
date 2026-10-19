package demodata

import (
	"time"
)

func (l *loader) schedule(gi, g, c int, trainer string, from, until, anchor time.Time, r *Result) {
	cid, gid, rid := id("club", c), id("group", gi), id("rule", gi)
	mode := "weekly"
	var weekdays []int32
	var parity any
	var dates []string
	days := []int32{1, 3, 5}
	if g/2 == 1 || g/2 == 3 || g/2 == 4 {
		days = []int32{2, 4, 6}
	}
	matches := func(d time.Time) bool {
		wd := int32(d.Weekday())
		if wd == 0 {
			wd = 7
		}
		for _, v := range days {
			if wd == v {
				return true
			}
		}
		return false
	}
	if g == 8 {
		mode = "day_parity"
		parity = "even"
		matches = func(d time.Time) bool { return d.Day()%2 == 0 }
	}
	for d := from; d.Before(until); d = d.AddDate(0, 0, 1) {
		if matches(d) {
			dates = append(dates, day(d))
		}
	}
	var selected any
	if g == 9 {
		mode = "date_list"
		selected = dates
	} else if mode == "weekly" {
		weekdays = days
	}
	times := []string{"16:00", "17:30", "16:00", "17:30", "19:00", "20:15", "19:00", "20:15", "13:00", "14:30"}
	l.exec(`INSERT INTO pride.schedule_rules(id,club_id,group_id,series_id,trainer_id,mode,weekdays,day_parity,selected_dates,starts_time,duration_minutes,timezone,valid_from,valid_until) VALUES($1,$2,$3,$1,$4,$5,$6,$7,$8::text[]::date[],$9::time,75,'Europe/Minsk',$10::date,$11::date)`, rid, cid, gid, trainer, mode, weekdays, parity, selected, times[g], day(from), day(until))
	// Ensure the first trainer in each club has a journal on the selected day,
	// including when it falls outside their recurring weekdays.
	todayFound := false
	for _, d := range dates {
		if d == day(anchor) {
			todayFound = true
		}
	}
	if g == 0 && !todayFound {
		dates = append(dates, day(anchor))
	}
	pastIndex, futureIndex := 0, 0
	for i, date := range dates {
		start, _ := time.ParseInLocation("2006-01-02 15:04", date+" "+times[g], anchor.Location())
		sid := id("session", gi*1000+i)
		var rule any = rid
		if !matches(start) {
			rule = nil
		}
		status := "planned"
		reason := ""
		actual := trainer
		manual := rule == nil
		if date < day(anchor) {
			if g == 1 && pastIndex == 0 {
				status = "cancelled"
				reason = "Зал закрыт на техническое обслуживание"
			} else {
				status = "closed"
			}
			if g == 0 && pastIndex == 1 {
				status = "planned"
			} // one overdue journal per club
			pastIndex++
		} else if date == day(anchor) && g == 0 {
			status = "open"
		} else if date > day(anchor) {
			if g == 0 && futureIndex == 0 {
				status = "cancelled"
				reason = "Соревнования воспитанников клуба"
			}
			if g == 2 && futureIndex == 0 {
				actual = id("trainer", c*5+4)
				manual = true
			}
			futureIndex++
		}
		var cancelled any
		var cancellationReason any
		if status == "cancelled" {
			cancelled = start.Add(-24 * time.Hour)
			if date > day(anchor) {
				cancelled = anchor.Add(-time.Hour)
			}
			cancellationReason = reason
			manual = true
		}
		l.exec(`INSERT INTO pride.training_sessions(id,club_id,group_id,schedule_rule_id,trainer_id,starts_on,timezone,starts_at,ends_at,status,is_manual_override,cancelled_at,cancellation_reason) VALUES($1,$2,$3,$4,$5,$6::date,'Europe/Minsk',$7,$8,'planned',$9,NULL,NULL)`, sid, cid, gid, rule, actual, date, start, start.Add(75*time.Minute), manual)
		if status == "cancelled" {
			l.exec(`UPDATE pride.training_sessions SET status='cancelled',cancelled_at=$2,cancellation_reason=$3 WHERE id=$1`, sid, cancelled, cancellationReason)
		}
		if status == "closed" || status == "open" {
			l.exec(`INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) SELECT club_id,$1,group_id,club_membership_id,id FROM pride.group_memberships WHERE group_id=$2`, sid, gid)
			l.exec(`UPDATE pride.training_sessions SET status='open',roster_frozen_at=$2 WHERE id=$1`, sid, start)
			// Deterministic variation by student and day: 80% present, with all absence
			// reasons represented. Today's journal is deliberately only partly marked.
			l.exec(`WITH marks AS (SELECT a.id,(row_number() OVER (ORDER BY p.full_name)+$2::int)%10 AS n,row_number() OVER (ORDER BY p.full_name) AS position FROM pride.attendance_records a JOIN pride.club_memberships m ON m.id=a.club_membership_id JOIN pride.participants p ON p.id=m.participant_id WHERE a.training_session_id=$1)
    UPDATE pride.attendance_records a SET state=CASE WHEN marks.n<8 THEN 'present' ELSE 'absent' END,
     absence_reason=CASE WHEN marks.n<8 THEN NULL WHEN marks.n=8 THEN 'illness' WHEN $2::int%2=0 THEN 'excused' ELSE 'unknown' END,
     marked_by=$3,marked_at=$4,origin='manual' FROM marks WHERE a.id=marks.id AND ($5::boolean OR marks.position<=8)`, sid, i, actual, start.Add(60*time.Minute), status == "closed")
			if status == "closed" {
				l.exec(`UPDATE pride.training_sessions SET status='closed',closed_at=$2 WHERE id=$1`, sid, start.Add(75*time.Minute))
				r.ClosedSessions++
			}
		}
		r.Sessions++
	}
	// A standalone weekend session demonstrates the fourth schedule mode.
	if g == 0 {
		date := anchor.AddDate(0, 0, 1)
		for date.Weekday() != time.Sunday {
			date = date.AddDate(0, 0, 1)
		}
		start := time.Date(date.Year(), date.Month(), date.Day(), 11, 0, 0, 0, anchor.Location())
		l.exec(`INSERT INTO pride.training_sessions(id,club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at,is_manual_override) VALUES($1,$2,$3,$4,$5::date,'Europe/Minsk',$6,$7,true)`, id("single_session", c), cid, gid, trainer, day(start), start, start.Add(90*time.Minute))
		r.Sessions++
	}
}
