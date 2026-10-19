package families

import (
	"net/http"
	"strings"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/platform"
	"github.com/jackc/pgx/v5"
)

func (s *Service) account(w http.ResponseWriter, r *http.Request) {
	// Reject non-managers before spending Argon2 resources.
	if _, err := s.auth.RequireClub(r.Context(), auth.CurrentUser(r.Context()).ID, r.PathValue("clubID"), "administrator"); err != nil {
		fail(w, err)
		return
	}
	creating := r.PathValue("userID") == ""
	revoke := strings.HasSuffix(r.URL.Path, "/revoke")
	var in struct {
		Login       *string `json:"login"`
		DisplayName *string `json:"displayName"`
		Password    *string `json:"password"`
		Version     *int    `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version == nil || *in.Version < 1 || (creating && (in.Login == nil || in.DisplayName == nil)) || (!creating && (in.Login != nil || in.DisplayName != nil)) || (revoke && in.Password != nil) || (!revoke && (in.Password == nil || len(*in.Password) < 12 || len(*in.Password) > 128)) {
		fail(w, errInput)
		return
	}
	login, name, hash := "", "", ""
	if creating {
		var err error
		login, err = auth.NormalizeLogin(*in.Login)
		name = strings.TrimSpace(*in.DisplayName)
		if err != nil || name == "" || !text(name, 200) {
			fail(w, errInput)
			return
		}
	}
	if !revoke {
		select {
		case s.hashing <- struct{}{}:
			defer func() { <-s.hashing }()
		default:
			w.Header().Set("Retry-After", "1")
			platform.WriteError(w, 429, "too_many_attempts", "Повторите действие чуть позже.")
			return
		}
		var err error
		hash, err = auth.HashPassword(*in.Password)
		*in.Password = ""
		if err != nil {
			fail(w, err)
			return
		}
	}
	var result Family
	err := s.transaction(r, true, !revoke, func(tx pgx.Tx) error {
		ctx := r.Context()
		f, err := loadFamily(ctx, tx, r.PathValue("clubID"), r.PathValue("familyID"), true)
		if err != nil {
			return err
		}
		if f.Version != *in.Version {
			return errConflict
		}
		user := r.PathValue("userID")
		action := "account_created"
		if creating {
			// An existing login cannot be attached or have its password overwritten by
			// this operation. A collision rolls back the entire family mutation.
			if err = tx.QueryRow(ctx, `INSERT INTO pride.staff_users(login,display_name,password_hash) VALUES($1,$2,$3) RETURNING id::text`, login, name, hash).Scan(&user); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO pride.family_access(family_id,user_id) VALUES($1,$2)`, f.ID, user); err != nil {
				return err
			}
		} else {
			var linked bool
			if err = tx.QueryRow(ctx, `SELECT true FROM pride.family_access WHERE family_id=$1 AND user_id::text=$2 FOR UPDATE`, f.ID, user).Scan(&linked); err != nil {
				return err
			}
			if revoke {
				action = "access_revoked"
				if _, err = tx.Exec(ctx, `DELETE FROM pride.family_access WHERE family_id=$1 AND user_id::text=$2`, f.ID, user); err != nil {
					return err
				}
			} else {
				action = "password_reset"
				var shared bool
				// Reset changes the human identity globally. A local administrator must
				// never gain another club's staff/owner identity through a family link.
				if err = tx.QueryRow(ctx, `SELECT u.is_owner OR EXISTS(SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id) OR EXISTS(SELECT 1 FROM pride.family_access a JOIN pride.families f ON f.id=a.family_id WHERE a.user_id=u.id AND f.club_id::text<>$2) FROM pride.staff_users u WHERE u.id::text=$1 AND u.disabled_at IS NULL FOR UPDATE OF u`, user, f.ClubID).Scan(&shared); err != nil {
					return err
				}
				var liveOwner bool
				if err = tx.QueryRow(ctx, `SELECT is_owner FROM pride.staff_users WHERE id=$1`, auth.CurrentUser(ctx).ID).Scan(&liveOwner); err != nil {
					return err
				}
				if shared && !liveOwner {
					return errSharedAccount
				}
				if _, err = tx.Exec(ctx, `UPDATE pride.staff_users SET password_hash=$2,version=version+1,updated_at=now() WHERE id::text=$1`, user, hash); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE pride.staff_sessions SET revoked_at=now() WHERE staff_user_id::text=$1 AND revoked_at IS NULL`, user); err != nil {
					return err
				}
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE pride.families SET version=version+1,updated_at=now() WHERE id=$1`, f.ID); err != nil {
			return err
		}
		result, err = loadFamily(ctx, tx, f.ClubID, f.ID, false)
		if err != nil {
			return err
		}
		return audit(ctx, tx, auth.CurrentUser(ctx).ID, result, action, user)
	})
	if err != nil {
		fail(w, err)
		return
	}
	status := 200
	if creating {
		status = 201
	}
	respond(w, status, result)
}
