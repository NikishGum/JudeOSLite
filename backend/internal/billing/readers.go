package billing

import (
	"net/http"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/families"
	"github.com/jackc/pgx/v5"
)

// Portal always requires explicit family access, even for parent+staff accounts.
// Retain link/club locks until all response data has been loaded; revoke/unlink
// then linearizes before or after this read, never halfway through it.
func memberScope(r *http.Request, tx pgx.Tx, portal bool) ([]string, error) {
	ids := []string{}
	if !portal {
		return ids, nil
	}
	ctx := r.Context()
	rows, err := tx.Query(ctx, `SELECT m.id::text,m.club_id::text FROM pride.family_access fa JOIN pride.family_children fc ON fc.family_id=fa.family_id JOIN pride.club_memberships m ON m.id=fc.membership_id JOIN pride.clubs c ON c.id=m.club_id WHERE fa.user_id=$1 AND c.archived_at IS NULL ORDER BY m.id FOR SHARE OF fa,fc,m,c`, auth.CurrentUser(ctx).ID)
	if err != nil {
		return nil, err
	}
	type member struct{ id, club string }
	members := []member{}
	for rows.Next() {
		var m member
		if err = rows.Scan(&m.id, &m.club); err != nil {
			break
		}
		members = append(members, m)
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	for _, m := range members {
		if err = families.LockMemberReader(ctx, tx, m.club, m.id); err != nil {
			return nil, err
		}
		ids = append(ids, m.id)
	}
	return ids, nil
}

type Subscription struct {
	ID           string `json:"id"`
	MembershipID string `json:"membershipId"`
	PaymentID    string `json:"paymentId"`
	PlanName     string `json:"planName"`
	Visits       int    `json:"visits"`
	Used         int    `json:"used"`
	ValidFrom    string `json:"validFrom"`
	ValidUntil   string `json:"validUntil"`
	Status       string `json:"status"`
}

const subscriptionJoins = ` FROM pride.subscriptions s JOIN pride.payments p ON p.id=s.payment_id JOIN pride.club_memberships m ON m.id=s.membership_id JOIN pride.participants n ON n.id=m.participant_id JOIN pride.clubs c ON c.id=s.club_id `
const subscriptionWhere = ` WHERE ($1='' OR s.club_id=nullif($1,'')::uuid) AND ($1<>'' OR s.membership_id=ANY($2::uuid[])) AND ($3='' OR s.membership_id=nullif($3,'')::uuid) AND ($4='' OR strpos(lower(n.full_name||' '||s.plan_name),lower($4))>0)`

func (s *Service) subscriptions(w http.ResponseWriter, r *http.Request) {
	f, err := query(r)
	if err != nil {
		fail(w, err)
		return
	}
	portal := r.URL.Path == "/api/portal/subscriptions"
	items := []Subscription{}
	total := 0
	err = s.transaction(r, portal, false, func(tx pgx.Tx) error {
		ctx := r.Context()
		members, err := memberScope(r, tx, portal)
		if err != nil {
			return err
		}
		args := []any{r.PathValue("clubID"), members, f.Member, f.Q}
		if err = tx.QueryRow(ctx, `SELECT count(*)`+subscriptionJoins+subscriptionWhere, args...).Scan(&total); err != nil {
			return err
		}
		args = append(args, f.Limit, f.Page*f.Limit)
		rows, err := tx.Query(ctx, `SELECT s.id::text,s.membership_id::text,s.payment_id::text,s.plan_name,s.visits,u.used,s.valid_from::text,s.valid_until::text,
   CASE WHEN s.cancelled_at IS NOT NULL OR p.status='void' THEN 'cancelled'
        WHEN p.status='pending' THEN 'pending'
        WHEN (now() AT TIME ZONE c.timezone)::date>=s.valid_until THEN 'expired'
        WHEN u.used>=s.visits THEN 'exhausted'
        WHEN (now() AT TIME ZONE c.timezone)::date<s.valid_from THEN 'pending' ELSE 'active' END`+subscriptionJoins+`
   CROSS JOIN LATERAL(SELECT count(*)::integer AS used FROM pride.attendance_records a JOIN pride.training_sessions t ON t.id=a.training_session_id
    WHERE a.club_membership_id=s.membership_id AND a.state='present' AND t.status='closed' AND t.superseded_at IS NULL AND (t.starts_at AT TIME ZONE t.timezone)::date>=s.valid_from AND (t.starts_at AT TIME ZONE t.timezone)::date<s.valid_until) u`+subscriptionWhere+` ORDER BY p.created_at DESC,s.id LIMIT $5 OFFSET $6`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v Subscription
			if err = rows.Scan(&v.ID, &v.MembershipID, &v.PaymentID, &v.PlanName, &v.Visits, &v.Used, &v.ValidFrom, &v.ValidUntil, &v.Status); err != nil {
				return err
			}
			items = append(items, v)
		}
		return rows.Err()
	})
	if err != nil {
		fail(w, err)
		return
	}
	listResponse(w, items, total, f.Page, f.Limit, len(items))
}

type Month struct {
	Month     string `json:"month"`
	PaidMinor int64  `json:"paidMinor"`
}
type Summary struct {
	PaidMinor    int64   `json:"paidMinor"`
	PendingMinor int64   `json:"pendingMinor"`
	PaidCount    int     `json:"paidCount"`
	PendingCount int     `json:"pendingCount"`
	Currency     string  `json:"currency"`
	ByMonth      []Month `json:"byMonth"`
}

func (s *Service) summary(w http.ResponseWriter, r *http.Request) {
	f, err := query(r)
	if err != nil {
		fail(w, err)
		return
	}
	result := Summary{Currency: "BYN", ByMonth: []Month{}}
	err = s.transaction(r, false, false, func(tx pgx.Tx) error {
		ctx := r.Context()
		args := []any{r.PathValue("clubID"), f.From, f.Until}
		where := ` FROM pride.payments p JOIN pride.clubs c ON c.id=p.club_id WHERE p.club_id=nullif($1,'')::uuid AND ($2='' OR coalesce(p.paid_on,(p.created_at AT TIME ZONE c.timezone)::date)>=nullif($2,'')::date) AND ($3='' OR coalesce(p.paid_on,(p.created_at AT TIME ZONE c.timezone)::date)<=nullif($3,'')::date)`
		if err := tx.QueryRow(ctx, `SELECT coalesce(sum(amount_minor) FILTER(WHERE status='paid'),0)::bigint,coalesce(sum(amount_minor) FILTER(WHERE status='pending'),0)::bigint,count(*) FILTER(WHERE status='paid'),count(*) FILTER(WHERE status='pending')`+where, args...).Scan(&result.PaidMinor, &result.PendingMinor, &result.PaidCount, &result.PendingCount); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT to_char(p.paid_on,'YYYY-MM'),sum(p.amount_minor)::bigint`+where+` AND p.status='paid' GROUP BY 1 ORDER BY 1`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m Month
			if err = rows.Scan(&m.Month, &m.PaidMinor); err != nil {
				return err
			}
			result.ByMonth = append(result.ByMonth, m)
		}
		return rows.Err()
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
