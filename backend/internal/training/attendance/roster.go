package attendance

import (
	"context"

	"github.com/NikishGum/JudeOSLite/backend/internal/training/sessions"
	"github.com/jackc/pgx/v5"
)

type RosterMember struct {
	MembershipID string `json:"membershipId"`
	Name         string `json:"fullName"`
	Archived     bool   `json:"archived"`
}
type Summary struct {
	Present  int `json:"present"`
	Absent   int `json:"absent"`
	Unmarked int `json:"unmarked"`
}
type Roster struct {
	Source  string         `json:"source"`
	Date    string         `json:"date"`
	Members []RosterMember `json:"members"`
	Summary *Summary       `json:"summary"`
}

// ReadRoster requires the caller's live club/trainer authorization and schedule
// and session locks. It never freezes a roster. Dated membership intervals match
// open exactly; once frozen, even an empty snapshot is authoritative.
func ReadRoster(ctx context.Context, tx pgx.Tx, v sessions.Session, records []Record) (Roster, error) {
	out := Roster{Source: "preliminary", Date: v.Date, Members: []RosterMember{}}
	if v.Frozen {
		out.Source = "snapshot"
		out.Summary = &Summary{}
		for _, r := range records {
			out.Members = append(out.Members, RosterMember{r.MembershipID, r.Name, r.Archived})
			switch {
			case r.State == nil:
				out.Summary.Unmarked++
			case *r.State == "present":
				out.Summary.Present++
			case *r.State == "absent":
				out.Summary.Absent++
			}
		}
		return out, nil
	}
	rows, e := tx.Query(ctx, `SELECT c.id::text,p.full_name,c.archived_at IS NOT NULL
		FROM pride.group_memberships m
		JOIN pride.club_memberships c ON c.id=m.club_membership_id
		JOIN pride.participants p ON p.id=c.participant_id
		WHERE m.club_id=$1 AND m.group_id=$2 AND m.valid_from<=$3::date
		AND (m.valid_until IS NULL OR $3::date<m.valid_until)
		ORDER BY p.full_name,m.id`, v.ClubID, v.GroupID, v.Date)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var m RosterMember
		if e = rows.Scan(&m.MembershipID, &m.Name, &m.Archived); e != nil {
			return out, e
		}
		out.Members = append(out.Members, m)
	}
	return out, rows.Err()
}
