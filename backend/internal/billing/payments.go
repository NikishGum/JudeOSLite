package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/jackc/pgx/v5"
)

type Payment struct {
	ID           string    `json:"id"`
	ClubID       string    `json:"clubId"`
	MembershipID string    `json:"membershipId"`
	FullName     string    `json:"fullName"`
	Description  string    `json:"description"`
	AmountMinor  int64     `json:"amountMinor"`
	Currency     string    `json:"currency"`
	Status       string    `json:"status"`
	Method       *string   `json:"method"`
	PaidOn       *string   `json:"paidOn"`
	CreatedAt    time.Time `json:"createdAt"`
	Version      int       `json:"version"`
	VoidReason   *string   `json:"voidReason"`
}
type paymentInput struct {
	RequestID    string  `json:"requestId"`
	MembershipID string  `json:"membershipId"`
	Description  string  `json:"description"`
	AmountMinor  int64   `json:"amountMinor"`
	Method       *string `json:"method"`
	PaidOn       *string `json:"paidOn"`
	PlanID       *string `json:"planId"`
	ValidFrom    *string `json:"validFrom"`
}

const paymentCols = `p.id::text,p.club_id::text,p.membership_id::text,n.full_name,p.description,p.amount_minor,p.currency,p.status,p.method,p.paid_on::text,p.created_at,p.version,p.void_reason`
const paymentJoins = ` FROM pride.payments p JOIN pride.club_memberships m ON m.id=p.membership_id JOIN pride.participants n ON n.id=m.participant_id JOIN pride.clubs c ON c.id=p.club_id `

func scanPayment(row pgx.Row) (Payment, error) {
	var p Payment
	err := row.Scan(&p.ID, &p.ClubID, &p.MembershipID, &p.FullName, &p.Description, &p.AmountMinor, &p.Currency, &p.Status, &p.Method, &p.PaidOn, &p.CreatedAt, &p.Version, &p.VoidReason)
	return p, err
}
func loadPayment(ctx context.Context, tx pgx.Tx, club, id string, lock bool) (Payment, error) {
	q := `SELECT ` + paymentCols + paymentJoins + ` WHERE p.club_id=nullif($1,'')::uuid AND p.id=$2::uuid`
	if lock {
		q += ` FOR UPDATE OF p`
	}
	return scanPayment(tx.QueryRow(ctx, q, club, id))
}
func (s *Service) createPayment(w http.ResponseWriter, r *http.Request) {
	var in paymentInput
	if !decode(w, r, &in, "requestId", "membershipId", "description", "amountMinor", "method", "paidOn", "planId", "validFrom") {
		return
	}
	if !uuid(in.RequestID) || !uuid(in.MembershipID) || strings.TrimSpace(in.Description) == "" || !validText(in.Description, 300) || in.AmountMinor < 1 || in.AmountMinor > 999999999 || (in.Method == nil) != (in.PaidOn == nil) || (in.Method != nil && (!method(*in.Method) || !date(*in.PaidOn))) || (in.PlanID == nil) != (in.ValidFrom == nil) || (in.PlanID != nil && (!uuid(*in.PlanID) || !date(*in.ValidFrom))) {
		fail(w, errInput)
		return
	}
	body, _ := json.Marshal(in)
	var result Payment
	err := s.transaction(r, false, true, func(tx pgx.Tx) error {
		ctx := r.Context()
		club := r.PathValue("clubID")
		// A transaction advisory lock serializes this UUID across all clubs, including
		// lost-response retries. Hash collisions only serialize unrelated requests.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1),202))`, in.RequestID); err != nil {
			return err
		}
		var id, existingClub string
		var same bool
		err := tx.QueryRow(ctx, `SELECT id::text,club_id::text,request_body=$2::jsonb FROM pride.payments WHERE request_id=$1`, in.RequestID, body).Scan(&id, &existingClub, &same)
		if err == nil {
			if !same || existingClub != club {
				return errConflict
			}
			result, err = loadPayment(ctx, tx, club, id, false)
			return err
		}
		if err != pgx.ErrNoRows {
			return err
		}
		var live bool
		if err = tx.QueryRow(ctx, `SELECT true FROM pride.club_memberships WHERE id=$1::uuid AND club_id=$2::uuid FOR UPDATE`, in.MembershipID, club).Scan(&live); err != nil {
			return err
		}
		var plan Plan
		var until string
		if in.PlanID != nil {
			plan, err = scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM pride.billing_plans WHERE id=$1::uuid AND club_id=$2::uuid FOR SHARE`, *in.PlanID, club))
			if err != nil {
				return err
			}
			if plan.Archived || plan.AmountMinor != in.AmountMinor {
				return errConflict
			}
			start, _ := time.Parse("2006-01-02", *in.ValidFrom)
			end := start.AddDate(0, 0, plan.Days)
			if end.Year() > 9999 {
				return errInput
			}
			until = end.Format("2006-01-02")
		}
		status := "pending"
		if in.Method != nil {
			status = "paid"
		}
		err = tx.QueryRow(ctx, `INSERT INTO pride.payments(club_id,membership_id,request_id,request_body,description,amount_minor,method,paid_on,status,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id::text`, club, in.MembershipID, in.RequestID, body, in.Description, in.AmountMinor, in.Method, in.PaidOn, status, auth.CurrentUser(ctx).ID).Scan(&id)
		if err != nil {
			return err
		}
		if in.PlanID != nil {
			if _, err = tx.Exec(ctx, `INSERT INTO pride.subscriptions(club_id,membership_id,payment_id,plan_name,visits,days,valid_from,valid_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, club, in.MembershipID, id, plan.Name, plan.Visits, plan.Days, *in.ValidFrom, until); err != nil {
				return err
			}
		}
		result, err = loadPayment(ctx, tx, club, id, false)
		if err != nil {
			return err
		}
		action := "payment_created"
		if status == "paid" {
			action = "payment_paid"
		}
		return audit(ctx, tx, club, id, "payment", action, nil, paymentState(result))
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 201, result)
}
func paymentState(p Payment) any {
	return map[string]any{"membershipId": p.MembershipID, "version": p.Version, "status": p.Status, "amountMinor": p.AmountMinor, "method": p.Method, "paidOn": p.PaidOn, "voidReason": p.VoidReason}
}
func (s *Service) changePayment(w http.ResponseWriter, r *http.Request) {
	confirm := strings.HasSuffix(r.URL.Path, "/confirm")
	var version *int
	var payMethod, paidOn, reason string
	if confirm {
		var in struct {
			Version *int   `json:"version"`
			Method  string `json:"method"`
			PaidOn  string `json:"paidOn"`
		}
		if !decode(w, r, &in) {
			return
		}
		version, payMethod, paidOn = in.Version, in.Method, in.PaidOn
		if !method(payMethod) || !date(paidOn) {
			fail(w, errInput)
			return
		}
	} else {
		var in struct {
			Version *int   `json:"version"`
			Reason  string `json:"reason"`
		}
		if !decode(w, r, &in) {
			return
		}
		version = in.Version
		reason = strings.TrimSpace(in.Reason)
		if reason == "" || !validText(reason, 500) {
			fail(w, errInput)
			return
		}
	}
	if version == nil || *version < 1 {
		fail(w, errInput)
		return
	}
	var result Payment
	err := s.transaction(r, false, true, func(tx pgx.Tx) error {
		ctx := r.Context()
		club, id := r.PathValue("clubID"), r.PathValue("paymentID")
		old, err := loadPayment(ctx, tx, club, id, true)
		if err != nil {
			return err
		}
		if old.Version != *version {
			return errConflict
		}
		// A current paid version may be confirmed again only with the same facts;
		// this is a no-op, with no duplicate audit or subscription.
		if confirm && old.Status == "paid" && old.Method != nil && old.PaidOn != nil && *old.Method == payMethod && *old.PaidOn == paidOn {
			result = old
			return nil
		}
		if old.Status == "void" || (confirm && old.Status != "pending") {
			return errConflict
		}
		action := "payment_paid"
		if confirm {
			_, err = tx.Exec(ctx, `UPDATE pride.payments SET status='paid',method=$2,paid_on=$3,version=version+1 WHERE id=$1`, old.ID, payMethod, paidOn)
		} else {
			action = "payment_voided"
			_, err = tx.Exec(ctx, `UPDATE pride.payments SET status='void',void_reason=$2,version=version+1 WHERE id=$1`, old.ID, reason)
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE pride.subscriptions SET cancelled_at=now() WHERE payment_id=$1 AND cancelled_at IS NULL`, old.ID)
			}
		}
		if err != nil {
			return err
		}
		result, err = loadPayment(ctx, tx, club, old.ID, false)
		if err != nil {
			return err
		}
		return audit(ctx, tx, club, old.ID, "payment", action, paymentState(old), paymentState(result))
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}

const paymentWhere = ` WHERE ($1='' OR p.club_id=nullif($1,'')::uuid) AND ($1<>'' OR p.membership_id=ANY($2::uuid[]))
 AND ($3='' OR strpos(lower(n.full_name||' '||p.description),lower($3))>0)
 AND ($4='' OR p.status=$4) AND ($5='' OR coalesce(p.paid_on,(p.created_at AT TIME ZONE c.timezone)::date)>=nullif($5,'')::date)
 AND ($6='' OR coalesce(p.paid_on,(p.created_at AT TIME ZONE c.timezone)::date)<=nullif($6,'')::date)
 AND ($7='' OR p.membership_id=nullif($7,'')::uuid)`

func (s *Service) payments(w http.ResponseWriter, r *http.Request) {
	f, err := query(r)
	if err != nil {
		fail(w, err)
		return
	}
	if f.Status != "" && f.Status != "pending" && f.Status != "paid" && f.Status != "void" {
		fail(w, errInput)
		return
	}
	portal := r.URL.Path == "/api/portal/payments"
	items := []Payment{}
	total := 0
	err = s.transaction(r, portal, false, func(tx pgx.Tx) error {
		members, err := memberScope(r, tx, portal)
		if err != nil {
			return err
		}
		args := []any{r.PathValue("clubID"), members, f.Q, f.Status, f.From, f.Until, f.Member}
		ctx := r.Context()
		if err = tx.QueryRow(ctx, `SELECT count(*)`+paymentJoins+paymentWhere, args...).Scan(&total); err != nil {
			return err
		}
		args = append(args, f.Limit, f.Page*f.Limit)
		rows, err := tx.Query(ctx, `SELECT `+paymentCols+paymentJoins+paymentWhere+` ORDER BY p.created_at DESC,p.id LIMIT $8 OFFSET $9`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			p, e := scanPayment(rows)
			if e != nil {
				return e
			}
			items = append(items, p)
		}
		return rows.Err()
	})
	if err != nil {
		fail(w, err)
		return
	}
	listResponse(w, items, total, f.Page, f.Limit, len(items))
}
