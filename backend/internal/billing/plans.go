package billing

import (
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
)

type Plan struct {
	ID          string `json:"id"`
	ClubID      string `json:"clubId"`
	Name        string `json:"name"`
	AmountMinor int64  `json:"amountMinor"`
	Visits      int    `json:"visits"`
	Days        int    `json:"days"`
	Archived    bool   `json:"archived"`
	Version     int    `json:"version"`
}
type planInput struct {
	Name        string `json:"name"`
	AmountMinor int64  `json:"amountMinor"`
	Visits      int    `json:"visits"`
	Days        int    `json:"days"`
	Archived    *bool  `json:"archived"`
	Version     *int   `json:"version"`
}

const planCols = `id::text,club_id::text,name,amount_minor,visits,days,archived,version`

func scanPlan(row pgx.Row) (Plan, error) {
	var p Plan
	err := row.Scan(&p.ID, &p.ClubID, &p.Name, &p.AmountMinor, &p.Visits, &p.Days, &p.Archived, &p.Version)
	return p, err
}
func (s *Service) plans(w http.ResponseWriter, r *http.Request) {
	f, err := query(r)
	if err != nil {
		fail(w, err)
		return
	}
	items := []Plan{}
	total := 0
	err = s.transaction(r, false, false, func(tx pgx.Tx) error {
		ctx := r.Context()
		club := r.PathValue("clubID")
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM pride.billing_plans WHERE club_id=nullif($1,'')::uuid AND ($2='' OR strpos(lower(name),lower($2))>0)`, club, f.Q).Scan(&total); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT `+planCols+` FROM pride.billing_plans WHERE club_id=nullif($1,'')::uuid AND ($2='' OR strpos(lower(name),lower($2))>0) ORDER BY lower(name),id LIMIT $3 OFFSET $4`, club, f.Q, f.Limit, f.Page*f.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			p, e := scanPlan(rows)
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
func (s *Service) savePlan(w http.ResponseWriter, r *http.Request) {
	var in planInput
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	create := r.Method == "POST"
	if in.Name == "" || !validText(in.Name, 200) || in.AmountMinor < 1 || in.AmountMinor > 999999999 || in.Visits < 1 || in.Visits > 366 || in.Days < 1 || in.Days > 366 || in.Version == nil || (create && (*in.Version != 0 || in.Archived != nil)) || (!create && (*in.Version < 1 || in.Archived == nil)) {
		fail(w, errInput)
		return
	}
	var result Plan
	err := s.transaction(r, false, true, func(tx pgx.Tx) error {
		ctx := r.Context()
		club, id := r.PathValue("clubID"), r.PathValue("planID")
		var before any
		action := "plan_created"
		if create {
			var err error
			result, err = scanPlan(tx.QueryRow(ctx, `INSERT INTO pride.billing_plans(club_id,name,amount_minor,visits,days) VALUES($1,$2,$3,$4,$5) RETURNING `+planCols, club, in.Name, in.AmountMinor, in.Visits, in.Days))
			if err != nil {
				return err
			}
		} else {
			old, err := scanPlan(tx.QueryRow(ctx, `SELECT `+planCols+` FROM pride.billing_plans WHERE id=$1::uuid AND club_id=$2::uuid FOR UPDATE`, id, club))
			if err != nil {
				return err
			}
			if old.Version != *in.Version {
				return errConflict
			}
			before = map[string]any{"version": old.Version}
			action = "plan_updated"
			result, err = scanPlan(tx.QueryRow(ctx, `UPDATE pride.billing_plans SET name=$3,amount_minor=$4,visits=$5,days=$6,archived=$7,version=version+1 WHERE id=$1::uuid AND club_id=$2::uuid RETURNING `+planCols, id, club, in.Name, in.AmountMinor, in.Visits, in.Days, *in.Archived))
			if err != nil {
				return err
			}
		}
		return audit(ctx, tx, club, result.ID, "billing_plan", action, before, map[string]any{"version": result.Version})
	})
	if err != nil {
		fail(w, err)
		return
	}
	status := 200
	if create {
		status = 201
	}
	respond(w, status, result)
}
