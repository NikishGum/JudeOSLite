package demodata

import (
	"encoding/json"
	"time"
)

func (l *loader) billing(child, c int, anchor time.Time) {
	plan := child % 3
	amounts := []int{8000, 11000, 14000}
	visits := []int{8, 12, 16}
	names := []string{"Базовый · 8 занятий", "Стандарт · 12 занятий", "Интенсив · 16 занятий"}
	for period := 0; period < 3; period++ {
		index := child*3 + period
		pid := id("payment", index)
		mid := id("member", child)
		from := anchor.AddDate(0, 0, -42+period*28)
		until := from.AddDate(0, 0, 28)
		status := "paid"
		var method, paid, voidReason, cancelled any
		if period == 2 || (period == 1 && child%10 == 0) {
			status = "pending"
		} else {
			method = []string{"cash", "card", "transfer", "erip"}[child%4]
			paid = day(from)
		}
		if period == 1 && child%20 == 1 {
			status = "void"
			voidReason = "Счёт аннулирован: выбран другой период занятий"
			cancelled = anchor.AddDate(0, 0, -1)
			method = nil
			paid = nil
		}
		description := "Занятия дзюдо · " + from.Format("02.01") + "–" + until.AddDate(0, 0, -1).Format("02.01.2006")
		request, _ := json.Marshal(map[string]any{"requestId": id("request", index), "membershipId": mid, "description": description, "amountMinor": amounts[plan], "method": method, "paidOn": paid, "planId": id("plan", c*3+plan), "validFrom": day(from)})
		created := from.AddDate(0, 0, -2)
		if created.After(anchor) {
			created = anchor.AddDate(0, 0, -1)
		}
		l.exec(`INSERT INTO pride.payments(id,club_id,membership_id,request_id,request_body,description,amount_minor,method,status,paid_on,void_reason,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::date,$11,$12,$13)`, pid, id("club", c), mid, id("request", index), request, description, amounts[plan], method, status, paid, voidReason, id("admin", c*2), created)
		l.exec(`INSERT INTO pride.subscriptions(id,club_id,membership_id,payment_id,plan_name,visits,days,valid_from,valid_until,cancelled_at) VALUES($1,$2,$3,$4,$5,$6,28,$7::date,$8::date,$9)`, id("subscription", index), id("club", c), mid, pid, names[plan], visits[plan], day(from), day(until), cancelled)
	}
}
