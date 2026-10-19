package families

import (
	"context"
	"net/http"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/jackc/pgx/v5"
)

// LockMemberReader is the common boundary for a specific membership's private
// profile, future billing/documents readers. Call inside auth.Require and retain
// the transaction until the response data is loaded. It rechecks live session,
// staff assignment or explicit family access, locking links against revocation.
// Club discovery and trainer membership alone never grant this permission.
func LockMemberReader(ctx context.Context, tx pgx.Tx, clubID, membershipID string) error {
	owner, err := auth.LockSession(ctx, tx)
	if err != nil {
		return err
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT true FROM pride.club_memberships WHERE club_id::text=$1 AND id::text=$2 FOR SHARE`, clubID, membershipID).Scan(&live); err != nil {
		return err
	}
	if owner {
		return nil
	}
	actor := auth.CurrentUser(ctx).ID
	err = tx.QueryRow(ctx, `SELECT true FROM pride.staff_club_roles WHERE staff_user_id=$1 AND club_id::text=$2 AND role='administrator' FOR SHARE`, actor, clubID).Scan(&live)
	if err == nil {
		return nil
	}
	if err != pgx.ErrNoRows {
		return err
	}
	return tx.QueryRow(ctx, `SELECT true FROM pride.family_children fc JOIN pride.family_access fa ON fa.family_id=fc.family_id JOIN pride.clubs c ON c.id=fc.club_id WHERE fc.club_id::text=$1 AND fc.membership_id::text=$2 AND fa.user_id=$3 AND c.archived_at IS NULL FOR SHARE OF fc,fa,c`, clubID, membershipID, actor).Scan(&live)
}

const memberCols = `m.id::text,m.club_id::text,p.full_name,p.birth_date::text,coalesce(cg.name,''),coalesce(cg.trainer_name,''),m.archived_at IS NOT NULL`
const currentGroup = `LEFT JOIN LATERAL (
 SELECT g.name,coalesce((SELECT u.display_name FROM pride.training_sessions s JOIN pride.staff_users u ON u.id=s.trainer_id
 WHERE s.group_id=g.id AND s.superseded_at IS NULL AND s.status<>'cancelled'
 ORDER BY (s.starts_at>=now()) DESC,abs(extract(epoch FROM s.starts_at-now())),s.id LIMIT 1),'') AS trainer_name
 FROM pride.group_memberships gm JOIN pride.training_groups g ON g.id=gm.group_id
 WHERE gm.club_membership_id=m.id AND gm.valid_from<=(now() AT TIME ZONE c.timezone)::date
 AND (gm.valid_until IS NULL OR gm.valid_until>(now() AT TIME ZONE c.timezone)::date)
 ORDER BY gm.valid_from DESC,gm.id LIMIT 1) cg ON true`

func scanMember(row pgx.Row, m *MemberSummary) error {
	return row.Scan(&m.ID, &m.ClubID, &m.FullName, &m.BirthDate, &m.GroupName, &m.TrainerName, &m.Archived)
}

type Group struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	TrainerName string  `json:"trainerName"`
	From        string  `json:"from"`
	Until       *string `json:"until"`
}
type Visit struct {
	ID            string  `json:"id"`
	SessionID     string  `json:"sessionId"`
	Date          string  `json:"date"`
	Time          string  `json:"time"`
	GroupName     string  `json:"groupName"`
	State         *string `json:"state"`
	AbsenceReason *string `json:"absenceReason"`
}
type Upcoming struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	GroupName   string `json:"groupName"`
	TrainerName string `json:"trainerName"`
}
type Attendance struct {
	Present int      `json:"present"`
	Absent  int      `json:"absent"`
	Rate    *float64 `json:"rate"`
	History []Visit  `json:"history"`
}
type MemberProfile struct {
	MemberSummary
	JoinedOn     string     `json:"joinedOn"`
	ContactName  string     `json:"contactName"`
	ContactValue string     `json:"contactValue"`
	FamilyID     *string    `json:"familyId"`
	FamilyName   *string    `json:"familyName"`
	Groups       []Group    `json:"groups"`
	Attendance   Attendance `json:"attendance"`
	Upcoming     []Upcoming `json:"upcoming"`
}

func (s *Service) profile(w http.ResponseWriter, r *http.Request) {
	result := MemberProfile{Groups: []Group{}, Attendance: Attendance{History: []Visit{}}, Upcoming: []Upcoming{}}
	err := s.transaction(r, false, false, func(tx pgx.Tx) error {
		ctx := r.Context()
		club, id := r.PathValue("clubID"), r.PathValue("membershipID")
		if err := LockMemberReader(ctx, tx, club, id); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT `+memberCols+`,m.joined_on::text,coalesce(m.contact_name,''),m.contact_value,f.id::text,f.name
  FROM pride.club_memberships m JOIN pride.participants p ON p.id=m.participant_id JOIN pride.clubs c ON c.id=m.club_id `+currentGroup+`
  LEFT JOIN pride.family_children fc ON fc.membership_id=m.id LEFT JOIN pride.families f ON f.id=fc.family_id
  WHERE m.id::text=$1 AND m.club_id::text=$2`, id, club).Scan(&result.ID, &result.ClubID, &result.FullName, &result.BirthDate, &result.GroupName, &result.TrainerName, &result.Archived, &result.JoinedOn, &result.ContactName, &result.ContactValue, &result.FamilyID, &result.FamilyName)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT g.id::text,g.name,coalesce((SELECT u.display_name FROM pride.training_sessions s JOIN pride.staff_users u ON u.id=s.trainer_id WHERE s.group_id=g.id AND s.superseded_at IS NULL AND s.status<>'cancelled' AND s.starts_on>=gm.valid_from AND (gm.valid_until IS NULL OR s.starts_on<gm.valid_until) ORDER BY s.starts_at DESC,s.id LIMIT 1),''),gm.valid_from::text,gm.valid_until::text
  FROM pride.group_memberships gm JOIN pride.training_groups g ON g.id=gm.group_id WHERE gm.club_membership_id::text=$1 ORDER BY gm.valid_from DESC,gm.id`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var g Group
			if err = rows.Scan(&g.ID, &g.Name, &g.TrainerName, &g.From, &g.Until); err != nil {
				break
			}
			result.Groups = append(result.Groups, g)
		}
		rows.Close()
		if err != nil {
			return err
		}
		if rows.Err() != nil {
			return rows.Err()
		}
		err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE a.state='present'),count(*) FILTER(WHERE a.state='absent') FROM pride.attendance_records a JOIN pride.training_sessions s ON s.id=a.training_session_id WHERE a.club_membership_id::text=$1 AND s.status='closed' AND s.superseded_at IS NULL`, id).Scan(&result.Attendance.Present, &result.Attendance.Absent)
		if err != nil {
			return err
		}
		if n := result.Attendance.Present + result.Attendance.Absent; n > 0 {
			v := 100 * float64(result.Attendance.Present) / float64(n)
			result.Attendance.Rate = &v
		}
		rows, err = tx.Query(ctx, `SELECT a.id::text,s.id::text,to_char(s.starts_at AT TIME ZONE s.timezone,'YYYY-MM-DD'),to_char(s.starts_at AT TIME ZONE s.timezone,'HH24:MI'),g.name,a.state,a.absence_reason
  FROM pride.attendance_records a JOIN pride.training_sessions s ON s.id=a.training_session_id JOIN pride.training_groups g ON g.id=s.group_id
  WHERE a.club_membership_id::text=$1 AND s.status IN ('open','closed') AND s.superseded_at IS NULL ORDER BY s.starts_at DESC,a.id LIMIT 20`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var v Visit
			if err = rows.Scan(&v.ID, &v.SessionID, &v.Date, &v.Time, &v.GroupName, &v.State, &v.AbsenceReason); err != nil {
				break
			}
			result.Attendance.History = append(result.Attendance.History, v)
		}
		rows.Close()
		if err != nil {
			return err
		}
		if rows.Err() != nil {
			return rows.Err()
		}
		rows, err = tx.Query(ctx, `SELECT s.id::text,to_char(s.starts_at AT TIME ZONE s.timezone,'YYYY-MM-DD'),to_char(s.starts_at AT TIME ZONE s.timezone,'HH24:MI'),g.name,u.display_name
  FROM pride.training_sessions s JOIN pride.training_groups g ON g.id=s.group_id JOIN pride.staff_users u ON u.id=s.trainer_id
  WHERE s.club_id::text=$2 AND s.status IN ('planned','open') AND s.superseded_at IS NULL AND s.ends_at>now() AND
  ((s.roster_frozen_at IS NOT NULL AND EXISTS(SELECT 1 FROM pride.attendance_records a WHERE a.training_session_id=s.id AND a.club_membership_id::text=$1)) OR
   (s.roster_frozen_at IS NULL AND EXISTS(SELECT 1 FROM pride.group_memberships gm JOIN pride.club_memberships m ON m.id=gm.club_membership_id WHERE gm.club_membership_id::text=$1 AND m.archived_at IS NULL AND gm.group_id=s.group_id AND gm.valid_from<=(s.starts_at AT TIME ZONE s.timezone)::date AND (gm.valid_until IS NULL OR gm.valid_until>(s.starts_at AT TIME ZONE s.timezone)::date)))) ORDER BY s.starts_at,s.id LIMIT 20`, id, club)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var u Upcoming
			if err = rows.Scan(&u.ID, &u.Date, &u.Time, &u.GroupName, &u.TrainerName); err != nil {
				return err
			}
			result.Upcoming = append(result.Upcoming, u)
		}
		return rows.Err()
	})
	if err != nil {
		fail(w, err)
		return
	}
	respond(w, 200, result)
}
