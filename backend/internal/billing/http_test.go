package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/NikishGum/JudeOSLite/backend/migrations"
)

func decodeResult[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func (f *fixture) base(club string) string { return "/api/clubs/" + club }
func (f *fixture) plan(visits, days int) Plan {
	f.t.Helper()
	w := f.call(f.admin, "POST", f.base(f.a)+"/billing/plans", map[string]any{"name": "Snapshot Plan", "amountMinor": 12345, "visits": visits, "days": days, "version": 0})
	f.status(w, 201)
	return decodeResult[Plan](f.t, w)
}
func (f *fixture) key() string { return f.id(`SELECT gen_random_uuid()::text`) }
func paymentBody(key, member string, plan *Plan, from string) map[string]any {
	b := map[string]any{"requestId": key, "membershipId": member, "description": "Synthetic invoice", "amountMinor": 12345, "method": nil, "paidOn": nil, "planId": nil, "validFrom": nil}
	if plan != nil {
		b["planId"] = plan.ID
		b["validFrom"] = from
	}
	return b
}
func (f *fixture) pay(b map[string]any) Payment {
	f.t.Helper()
	w := f.call(f.admin, "POST", f.base(f.a)+"/payments", b)
	f.status(w, 201)
	return decodeResult[Payment](f.t, w)
}
func (f *fixture) subs(c client, scope string) []Subscription {
	f.t.Helper()
	w := f.call(c, "GET", scope, nil)
	f.status(w, 200)
	v := decodeResult[struct {
		Items []Subscription
		Total int
	}](f.t, w)
	if v.Items == nil {
		f.t.Fatal("null list")
	}
	return v.Items
}

func TestManualMoneyIdempotencyVersionsAndSummary(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Money Child")
	base := f.base(f.a)
	b := paymentBody(f.key(), m, nil, "")
	p := f.pay(b)
	if p.Status != "pending" || p.Method != nil || p.PaidOn != nil || p.VoidReason != nil || p.Currency != "BYN" || p.AmountMinor != 12345 || p.Version != 1 {
		t.Fatal(p)
	}
	if got := f.pay(b); got.ID != p.ID {
		t.Fatal("duplicate payment")
	}
	b["amountMinor"] = 12346
	f.status(f.call(f.admin, "POST", base+"/payments", b), 409)
	b["amountMinor"] = 12345
	endpoint := base + "/payments/" + p.ID
	f.status(f.call(f.admin, "POST", endpoint+"/confirm", map[string]any{"method": "cash", "paidOn": "2026-10-01"}), 400)
	confirm := map[string]any{"version": 1, "method": "erip", "paidOn": "2026-10-01"}
	w := f.call(f.admin, "POST", endpoint+"/confirm", confirm)
	f.status(w, 200)
	p = decodeResult[Payment](t, w)
	if p.Status != "paid" || p.Version != 2 || *p.Method != "erip" {
		t.Fatal(p)
	}
	f.status(f.call(f.admin, "POST", endpoint+"/confirm", confirm), 409)
	confirm["version"] = 2
	w = f.call(f.admin, "POST", endpoint+"/confirm", confirm)
	f.status(w, 200)
	if decodeResult[Payment](t, w).Version != 2 {
		t.Fatal("no-op increment")
	}
	confirm["method"] = "cash"
	f.status(f.call(f.admin, "POST", endpoint+"/confirm", confirm), 409)
	// Retry the creation after confirmation: returns current state, no new audit.
	if got := f.pay(b); got.ID != p.ID || got.Status != "paid" || got.Version != 2 {
		t.Fatal(got)
	}
	if n := f.id(`SELECT count(*)::text FROM pride.audit_events WHERE entity_id=$1`, p.ID); n != "2" {
		t.Fatal(n)
	}
	cash := paymentBody(f.key(), m, nil, "")
	cash["amountMinor"] = 999999999
	cash["method"] = "card"
	cash["paidOn"] = "2026-09-30"
	f.pay(cash)
	pending := paymentBody(f.key(), m, nil, "")
	pending["amountMinor"] = 1
	f.pay(pending)
	getSum := func(suffix string) Summary {
		w := f.call(f.admin, "GET", base+"/billing/summary"+suffix, nil)
		f.status(w, 200)
		return decodeResult[Summary](t, w)
	}
	sum := getSum("")
	if sum.PaidMinor != 1000012344 || sum.PendingMinor != 1 || sum.PaidCount != 2 || sum.PendingCount != 1 || len(sum.ByMonth) != 2 {
		t.Fatal(sum)
	}
	sum = getSum("?from=2026-10-01&until=2026-10-01")
	if sum.PaidMinor != 12345 || sum.PaidCount != 1 {
		t.Fatal(sum)
	}
	f.status(f.call(f.admin, "POST", endpoint+"/void", map[string]any{"version": 2, "reason": " "}), 400)
	w = f.call(f.admin, "POST", endpoint+"/void", map[string]any{"version": 2, "reason": "Synthetic correction"})
	f.status(w, 200)
	p = decodeResult[Payment](t, w)
	if p.Status != "void" || p.Version != 3 || p.VoidReason == nil {
		t.Fatal(p)
	}
	f.status(f.call(f.admin, "POST", endpoint+"/void", map[string]any{"version": 3, "reason": "again"}), 409)
	f.status(f.call(f.admin, "POST", endpoint+"/confirm", map[string]any{"version": 3, "method": "cash", "paidOn": "2026-10-01"}), 409)
	if sum = getSum(""); sum.PaidMinor != 999999999 || sum.PaidCount != 1 {
		t.Fatal(sum)
	}
	if err := migrations.Apply(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	// q/status/date/member filters, stable disjoint pages and mandatory null fields.
	w = f.call(f.admin, "GET", base+"/payments?status=void&q=Money&from=2026-10-01&until=2026-10-01&membershipId="+m, nil)
	f.status(w, 200)
	list := decodeResult[struct {
		Items   []Payment
		Total   int
		HasMore bool
	}](t, w)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != p.ID {
		t.Fatal(w.Body.String())
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		w = f.call(f.admin, "GET", fmt.Sprintf("%s/payments?limit=1&page=%d", base, i), nil)
		f.status(w, 200)
		v := decodeResult[struct {
			Items   []Payment
			Total   int
			HasMore bool
		}](t, w)
		if v.Total != 3 || len(v.Items) != 1 || seen[v.Items[0].ID] || v.HasMore != (i < 2) {
			t.Fatal(w.Body.String())
		}
		seen[v.Items[0].ID] = true
	}
}

func TestConcurrentRequestConfirmAndOverlappingIntervals(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Concurrent")
	plan := f.plan(2, 30)
	b := paymentBody(f.key(), m, &plan, "2026-10-01")
	base := f.base(f.a)
	run := func(n int, fn func(int) *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
		out := make([]*httptest.ResponseRecorder, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range out {
			wg.Add(1)
			go func(i int) { defer wg.Done(); <-start; out[i] = fn(i) }(i)
		}
		close(start)
		wg.Wait()
		return out
	}
	responses := run(8, func(int) *httptest.ResponseRecorder { return f.call(f.admin, "POST", base+"/payments", b) })
	id := ""
	for _, w := range responses {
		f.status(w, 201)
		p := decodeResult[Payment](t, w)
		if id != "" && id != p.ID {
			t.Fatal("idempotency race")
		}
		id = p.ID
	}
	if n := f.id(`SELECT count(*)::text FROM pride.subscriptions`); n != "1" {
		t.Fatal(n)
	}
	if n := f.id(`SELECT count(*)::text FROM pride.audit_events WHERE entity_type='payment'`); n != "1" {
		t.Fatal(n)
	}
	confirm := map[string]any{"version": 1, "method": "cash", "paidOn": "2026-10-01"}
	ok, conflict := 0, 0
	for _, w := range run(2, func(int) *httptest.ResponseRecorder {
		return f.call(f.admin, "POST", base+"/payments/"+id+"/confirm", confirm)
	}) {
		switch w.Code {
		case 200:
			ok++
		case 409:
			conflict++
		default:
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	f.status(f.call(f.admin, "POST", base+"/payments", paymentBody(f.key(), m, &plan, "2026-10-30")), 409)
	// Adjacent [from,until) is allowed; competing requests for that next interval
	// produce one success and one conflict, with no orphan payment/audit.
	bodies := []map[string]any{paymentBody(f.key(), m, &plan, "2026-10-31"), paymentBody(f.key(), m, &plan, "2026-10-31")}
	ok, conflict = 0, 0
	for _, w := range run(2, func(i int) *httptest.ResponseRecorder { return f.call(f.admin, "POST", base+"/payments", bodies[i]) }) {
		switch w.Code {
		case 201:
			ok++
		case 409:
			conflict++
		default:
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	if n := f.id(`SELECT count(*)::text FROM pride.payments`); n != "2" {
		t.Fatal(n)
	}
	if n := f.id(`SELECT count(*)::text FROM pride.audit_events WHERE entity_type='payment'`); n != "3" {
		t.Fatal(n)
	}
	f.status(f.call(f.admin, "POST", base+"/payments/"+id+"/void", map[string]any{"version": 2, "reason": "Cancel overlap"}), 200)
	f.pay(paymentBody(f.key(), m, &plan, "2026-10-01"))
}

func TestTariffSnapshotUsageBoundariesAndCorrection(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Visits")
	p := f.plan(1, 30)
	base := f.base(f.a)
	b := paymentBody(f.key(), m, &p, "2026-10-01")
	b["method"] = "transfer"
	b["paidOn"] = "2026-10-01"
	paid := f.pay(b)
	update := map[string]any{"name": "Changed Plan", "amountMinor": 20000, "visits": 9, "days": 10, "archived": true, "version": 1}
	w := f.call(f.admin, "PUT", base+"/billing/plans/"+p.ID, update)
	f.status(w, 200)
	f.status(f.call(f.admin, "PUT", base+"/billing/plans/"+p.ID, update), 409)
	if retry := f.pay(b); retry.ID != paid.ID {
		t.Fatal("source tariff edit changed idempotent creation")
	}
	f.status(f.call(f.admin, "POST", base+"/payments", paymentBody(f.key(), m, &p, "2026-11-01")), 409)
	g := f.id(`INSERT INTO pride.training_groups(club_id,name) VALUES($1,'Usage') RETURNING id::text`, f.a)
	source := f.id(`INSERT INTO pride.group_memberships(club_id,group_id,club_membership_id,valid_from) VALUES($1,$2,$3,'2020-01-01') RETURNING id::text`, f.a, g, m)
	records := []string{}
	sessions := []string{}
	for i, day := range []string{"2026-09-30", "2026-10-01", "2026-10-30", "2026-10-31", "2026-10-10", "2026-10-11", "2026-10-12"} {
		session := f.id(`INSERT INTO pride.training_sessions(club_id,group_id,trainer_id,starts_on,timezone,starts_at,ends_at) VALUES($1,$2,$3,$4::date,'Europe/Minsk',$4::date+time '18:00' AT TIME ZONE 'Europe/Minsk',$4::date+time '19:00' AT TIME ZONE 'Europe/Minsk') RETURNING id::text`, f.a, g, f.trainerID, day)
		record := f.id(`INSERT INTO pride.attendance_records(club_id,training_session_id,group_id,club_membership_id,source_group_membership_id) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, f.a, session, g, m, source)
		records = append(records, record)
		sessions = append(sessions, session)
		f.exec(`UPDATE pride.training_sessions SET status='open',roster_frozen_at=now() WHERE id=$1`, session)
		state, reason := "present", any(nil)
		if i == 6 {
			state, reason = "absent", "illness"
		}
		f.exec(`UPDATE pride.attendance_records SET state=$2,absence_reason=$3,marked_by=$4,marked_at=now(),origin='manual' WHERE id=$1`, record, state, reason, f.ownerID)
		if i == 4 {
			f.exec(`UPDATE pride.training_sessions SET status='cancelled',cancelled_at=now(),cancellation_reason='Synthetic' WHERE id=$1`, session)
		} else if i != 5 {
			f.exec(`UPDATE pride.training_sessions SET status='closed',closed_at=now() WHERE id=$1`, session)
		}
	}
	read := func() Subscription {
		v := f.subs(f.admin, base+"/subscriptions?membershipId="+m)
		if len(v) != 1 {
			t.Fatal(v)
		}
		return v[0]
	}
	sub := read()
	if sub.PlanName != "Snapshot Plan" || sub.Visits != 1 || sub.ValidUntil != "2026-10-31" || sub.Used != 2 {
		t.Fatal(sub)
	}
	if days := f.id(`SELECT days::text FROM pride.subscriptions WHERE id=$1`, sub.ID); days != "30" {
		t.Fatal(days)
	}
	// Corrections use the real existing HTTP endpoint, not a parallel billing ledger.
	w = f.call(f.admin, "PUT", base+"/journals/"+sessions[1]+"/records/"+records[1], map[string]any{"version": 1, "state": "absent", "absenceReason": "excused", "changeReason": "Synthetic usage correction"})
	// The fixture SQL retains version1; HTTP corrections increment it.
	if w.Code != 200 {
		t.Log(w.Body.String())
		t.Fatal("correction endpoint contract changed")
	}
	if sub = read(); sub.Used != 1 {
		t.Fatal(sub)
	}
	w = f.call(f.admin, "PUT", base+"/journals/"+sessions[6]+"/records/"+records[6], map[string]any{"version": 1, "state": "present", "absenceReason": nil, "changeReason": "Synthetic present correction"})
	f.status(w, 200)
	if sub = read(); sub.Used != 2 {
		t.Fatal(sub)
	}
	f.status(f.call(f.admin, "POST", base+"/payments/"+paid.ID+"/void", map[string]any{"version": 1, "reason": "Synthetic cancellation"}), 200)
	if sub = read(); sub.Status != "cancelled" || sub.Used != 2 {
		t.Fatal(sub)
	}
}

func TestPermissionsFamiliesRevocationAndInput(t *testing.T) {
	f := setup(t)
	a := f.member(f.a, "Same Child")
	b := f.member(f.a, "Same Child")
	foreign := f.member(f.b, "Same Child")
	base := f.base(f.a)
	paid := f.pay(paymentBody(f.key(), a, nil, ""))
	f.pay(paymentBody(f.key(), b, nil, ""))
	fam := f.id(`INSERT INTO pride.families(club_id,name) VALUES($1,'Family A') RETURNING id::text`, f.a)
	f.exec(`INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3)`, fam, f.a, a)
	// parent+trainer exercises the union without letting trainer rights broaden portal.
	f.exec(`INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, fam, f.trainerID)
	portal := f.login("trainer", password)
	w := f.call(portal, "GET", "/api/portal/payments", nil)
	f.status(w, 200)
	v := decodeResult[struct {
		Items []Payment
		Total int
	}](t, w)
	if v.Total != 1 || v.Items[0].ID != paid.ID {
		t.Fatal(w.Body.String())
	}
	for _, id := range []string{b, foreign} {
		w = f.call(portal, "GET", "/api/portal/payments?membershipId="+id, nil)
		f.status(w, 200)
		if !strings.Contains(w.Body.String(), `"total":0`) {
			t.Fatal(w.Body.String())
		}
	}
	for _, suffix := range []string{"/payments", "/subscriptions", "/billing/plans", "/billing/summary"} {
		f.status(f.call(f.trainer, "GET", base+suffix, nil), 404)
		f.status(f.call(client{}, "GET", base+suffix, nil), 401)
		f.status(f.call(f.admin, "GET", f.base(f.b)+suffix, nil), 404)
	}
	f.status(f.call(portal, "POST", base+"/payments", paymentBody(f.key(), a, nil, "")), 404)
	f.status(f.call(f.admin, "POST", base+"/payments", paymentBody(f.key(), foreign, nil, "")), 404)
	noCSRF := f.admin
	noCSRF.csrf = ""
	f.status(f.call(noCSRF, "POST", base+"/payments", paymentBody(f.key(), a, nil, "")), 403)
	for _, change := range []map[string]any{{"amountMinor": 0}, {"amountMinor": 1000000000}, {"amountMinor": 1.5}, {"method": "ipay", "paidOn": "2026-10-01"}, {"method": "cash"}, {"paidOn": "2026-10-01"}, {"method": "cash", "paidOn": "2026-02-30"}, {"planId": f.key()}, {"validFrom": "2026-10-01"}, {"requestId": "invalid"}, {"description": "\n"}} {
		in := paymentBody(f.key(), a, nil, "")
		for k, v := range change {
			in[k] = v
		}
		f.status(f.call(f.admin, "POST", base+"/payments", in), 400)
	}
	for _, key := range []string{"method", "paidOn", "planId", "validFrom"} {
		in := paymentBody(f.key(), a, nil, "")
		delete(in, key)
		f.status(f.call(f.admin, "POST", base+"/payments", in), 400)
	}
	for _, q := range []string{"limit=0", "limit=101", "page=-1", "page=x", "status=unknown", "from=2026-02-30", "from=2026-10-02&until=2026-10-01", "membershipId=bad"} {
		f.status(f.call(f.admin, "GET", base+"/payments?"+q, nil), 400)
	}
	f.exec(`DELETE FROM pride.family_children WHERE membership_id=$1`, a)
	w = f.call(portal, "GET", "/api/portal/payments", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal(w.Body.String())
	}
	f.exec(`INSERT INTO pride.family_children(family_id,club_id,membership_id) VALUES($1,$2,$3)`, fam, f.a, a)
	f.exec(`DELETE FROM pride.family_access WHERE user_id=$1`, f.trainerID)
	w = f.call(portal, "GET", "/api/portal/payments", nil)
	f.status(w, 200)
	if !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatal(w.Body.String())
	}
	f.exec(`DELETE FROM pride.staff_club_roles WHERE staff_user_id=$1`, f.adminID)
	f.status(f.call(f.admin, "GET", base+"/billing/summary", nil), 404)
	f.exec(`UPDATE pride.staff_users SET disabled_at=now() WHERE id=$1`, f.trainerID)
	f.status(f.call(portal, "GET", "/api/portal/payments", nil), 401)
	f.exec(`UPDATE pride.clubs SET archived_at=now() WHERE id=$1`, f.a)
	f.status(f.call(f.owner, "POST", base+"/payments", paymentBody(f.key(), a, nil, "")), 409)
}

func TestSubscriptionCalendarStatusesAndPlanValidation(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Calendar")
	p := f.plan(1, 1)
	base := f.base(f.a)
	today := f.id(`SELECT to_char(now() AT TIME ZONE 'Europe/Minsk','YYYY-MM-DD')`)
	yesterday := f.id(`SELECT to_char((now() AT TIME ZONE 'Europe/Minsk')::date-1,'YYYY-MM-DD')`)
	tomorrow := f.id(`SELECT to_char((now() AT TIME ZONE 'Europe/Minsk')::date+1,'YYYY-MM-DD')`)
	for _, day := range []string{yesterday, today, tomorrow} {
		b := paymentBody(f.key(), m, &p, day)
		b["method"] = "cash"
		b["paidOn"] = today
		f.pay(b)
	}
	subs := f.subs(f.admin, base+"/subscriptions")
	states := map[string]string{}
	for _, sub := range subs {
		states[sub.ValidFrom] = sub.Status
	}
	if states[yesterday] != "expired" || states[today] != "active" || states[tomorrow] != "pending" {
		t.Fatal(states)
	}
	// Pending reserves its interval, including after its end, until explicit void.
	another := f.member(f.a, "Pending")
	f.pay(paymentBody(f.key(), another, &p, yesterday))
	subs = f.subs(f.admin, base+"/subscriptions?membershipId="+another)
	if len(subs) != 1 || subs[0].Status != "pending" {
		t.Fatal(subs)
	}
	for _, change := range []map[string]any{{"visits": 0}, {"visits": 367}, {"days": 0}, {"days": 367}, {"amountMinor": 0}, {"amountMinor": 1000000000}, {"name": "\t"}, {"version": nil}, {"archived": true}} {
		in := map[string]any{"name": "Valid", "amountMinor": 1, "visits": 1, "days": 1, "version": 0}
		for k, v := range change {
			in[k] = v
		}
		f.status(f.call(f.admin, "POST", base+"/billing/plans", in), 400)
	}
	f.status(f.call(f.admin, "POST", base+"/payments", paymentBody(f.key(), another, &p, "9999-12-31")), 400)
	wrongPrice := paymentBody(f.key(), another, &p, today)
	wrongPrice["amountMinor"] = 12344
	f.status(f.call(f.admin, "POST", base+"/payments", wrongPrice), 409)
	w := f.call(f.admin, "GET", base+"/billing/plans?limit=1&page=0&q=Snapshot", nil)
	f.status(w, 200)
	plans := decodeResult[struct {
		Items   []Plan
		Total   int
		HasMore bool
	}](t, w)
	if plans.Total != 1 || len(plans.Items) != 1 || plans.HasMore {
		t.Fatal(w.Body.String())
	}
	f.status(f.call(f.admin, "GET", base+"/payments?status=all", nil), 200)
}

func TestBillingAuditFailureRollsBackWholeCommand(t *testing.T) {
	f := setup(t)
	m := f.member(f.a, "Atomic")
	plan := f.plan(8, 30)
	base := f.base(f.a)
	b := paymentBody(f.key(), m, &plan, "2026-10-01")
	p := f.pay(b)
	f.exec(`CREATE FUNCTION pride.reject_billing_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic audit failure'; END $$`)
	f.exec(`CREATE TRIGGER reject_billing_audit BEFORE INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.reject_billing_audit()`)
	f.status(f.call(f.admin, "POST", base+"/payments/"+p.ID+"/confirm", map[string]any{"version": 1, "method": "cash", "paidOn": "2026-10-01"}), 503)
	f.status(f.call(f.admin, "POST", base+"/payments/"+p.ID+"/void", map[string]any{"version": 1, "reason": "Synthetic rejected audit"}), 503)
	newBody := paymentBody(f.key(), f.member(f.a, "New atomic"), &plan, "2026-10-01")
	f.status(f.call(f.admin, "POST", base+"/payments", newBody), 503)
	if state := f.id(`SELECT status||':'||version::text FROM pride.payments WHERE id=$1`, p.ID); state != "pending:1" {
		t.Fatal(state)
	}
	if n := f.id(`SELECT count(*)::text FROM pride.payments`); n != "1" {
		t.Fatal(n)
	}
	if n := f.id(`SELECT count(*)::text FROM pride.subscriptions WHERE cancelled_at IS NULL`); n != "1" {
		t.Fatal(n)
	}
	f.status(f.call(f.admin, "PUT", base+"/billing/plans/"+plan.ID, map[string]any{"name": "Rejected", "amountMinor": 1, "visits": 1, "days": 1, "archived": true, "version": 1}), 503)
	if state := f.id(`SELECT name||':'||version::text FROM pride.billing_plans WHERE id=$1`, plan.ID); state != "Snapshot Plan:1" {
		t.Fatal(state)
	}
	f.exec(`DROP TRIGGER reject_billing_audit ON pride.audit_events`)
	// A failed create did not consume its idempotency key.
	f.pay(newBody)
}
