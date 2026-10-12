package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/NikishGum/JudeOSLite/backend/internal/auth"
	"github.com/NikishGum/JudeOSLite/backend/internal/clubs"
	"github.com/NikishGum/JudeOSLite/backend/internal/training/groups"
	"github.com/jackc/pgx/v5"
)

// Generate is safe on startup, retries and simultaneous instances. It does not
// rewrite any existing session. Revoked/archived assignments retain history and
// stop extending the horizon until the administrator creates a new version.
func (s *Service) Generate(ctx context.Context, now time.Time) error {
	rows, e := s.pool.Query(ctx, `SELECT id::text FROM pride.schedule_rules ORDER BY id`)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	var result error
	for _, id := range ids {
		e = s.generateRule(ctx, id, now)
		if errors.Is(e, pgx.ErrNoRows) || errors.Is(e, clubs.ErrArchived) || errors.Is(e, groups.ErrArchived) {
			continue
		}
		if e != nil {
			result = errors.Join(result, e)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return result
}
func (s *Service) generateRule(ctx context.Context, id string, now time.Time) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if e = lock(ctx, tx); e != nil {
		return e
	}
	rule, e := scanRule(tx.QueryRow(ctx, `SELECT `+ruleColumns+` FROM pride.schedule_rules WHERE id=$1`, id))
	if e != nil {
		return e
	}
	if e = clubs.RequireActive(ctx, tx, rule.ClubID); e != nil {
		return e
	}
	if _, e = groups.ReadGroup(ctx, tx, rule.ClubID, rule.GroupID, true); e != nil {
		return e
	}
	if e = auth.RequireTrainer(ctx, tx, rule.TrainerID, rule.ClubID); e != nil {
		return e
	}
	dates, e := expand(rule.Input, rule.Timezone, now)
	if e != nil {
		return e
	}
	var conflict error
	for _, o := range dates {
		var preserved bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pride.training_sessions s JOIN pride.schedule_rules r ON r.id=s.schedule_rule_id WHERE r.series_id=$1 AND s.starts_on=$2::date AND s.superseded_at IS NULL AND (s.status<>'planned' OR s.is_manual_override OR s.roster_frozen_at IS NOT NULL))`, rule.SeriesID, o.Date).Scan(&preserved)
		if e != nil {
			return e
		}
		if preserved {
			continue
		}
		var existingRule *string
		e = tx.QueryRow(ctx, `SELECT schedule_rule_id::text FROM pride.training_sessions WHERE group_id=$1 AND starts_at=$2 AND superseded_at IS NULL`, rule.GroupID, o.StartsAt).Scan(&existingRule)
		if e == nil {
			if existingRule != nil && *existingRule == rule.ID {
				continue
			}
			// A later one-off can occupy a slot outside the original preview
			// horizon. Keep that slot and still extend the other dates.
			conflict = ErrDuplicate
			continue
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if _, e = insertSession(ctx, tx, rule.ClubID, rule.ID, rule.Input, rule.Timezone, o); e != nil {
			return e
		}
	}
	if e := tx.Commit(ctx); e != nil {
		return e
	}
	return conflict
}
