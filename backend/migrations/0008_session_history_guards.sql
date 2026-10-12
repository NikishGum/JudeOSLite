-- Reuse V1-07 sessions. Protect source occurrence identity and terminal history;
-- snapshots/attendance are added by V1-09 and remain separate immutable authors.
CREATE FUNCTION pride.guard_session_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.id,NEW.club_id,NEW.group_id,NEW.schedule_rule_id,NEW.starts_on,NEW.timezone,NEW.created_at)
        IS DISTINCT FROM (OLD.id,OLD.club_id,OLD.group_id,OLD.schedule_rule_id,OLD.starts_on,OLD.timezone,OLD.created_at) THEN
        RAISE EXCEPTION 'session source identity is immutable' USING ERRCODE='23514';
    END IF;
    IF (OLD.superseded_at IS NOT NULL OR OLD.status='cancelled') AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'retired or cancelled session is immutable' USING ERRCODE='23514';
    END IF;
    IF OLD.status='closed' AND (NEW.status,NEW.trainer_id,NEW.starts_at,NEW.ends_at,NEW.closed_at,NEW.cancelled_at,NEW.cancellation_reason)
        IS DISTINCT FROM (OLD.status,OLD.trainer_id,OLD.starts_at,OLD.ends_at,OLD.closed_at,OLD.cancelled_at,OLD.cancellation_reason) THEN
        RAISE EXCEPTION 'closed session history is immutable' USING ERRCODE='23514';
    END IF;
    IF (OLD.status='open' OR OLD.roster_frozen_at IS NOT NULL) AND (NEW.starts_at,NEW.ends_at)
        IS DISTINCT FROM (OLD.starts_at,OLD.ends_at) THEN
        RAISE EXCEPTION 'opened session time is immutable' USING ERRCODE='23514';
    END IF;
    IF OLD.roster_frozen_at IS NOT NULL AND NEW.roster_frozen_at IS DISTINCT FROM OLD.roster_frozen_at THEN
        RAISE EXCEPTION 'session roster freeze is immutable' USING ERRCODE='23514';
    END IF;
    IF OLD.is_manual_override AND NOT NEW.is_manual_override THEN
        RAISE EXCEPTION 'manual exception cannot be cleared' USING ERRCODE='23514';
    END IF;
    IF OLD.status='open' AND NEW.status NOT IN ('open','closed','cancelled') THEN
        RAISE EXCEPTION 'opened session cannot become planned' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER guard_session_history BEFORE UPDATE ON pride.training_sessions
    FOR EACH ROW EXECUTE FUNCTION pride.guard_session_history();
