ALTER TABLE pride.attendance_records ADD COLUMN change_reason text;
ALTER TABLE pride.attendance_records DROP CONSTRAINT attendance_records_check;
ALTER TABLE pride.attendance_records ADD CONSTRAINT attendance_value CHECK ((
  (state IS NULL AND absence_reason IS NULL AND marked_by IS NULL AND marked_at IS NULL AND origin IS NULL AND change_reason IS NULL)
  OR (state IN ('present','absent') AND marked_by IS NOT NULL AND marked_at IS NOT NULL
    AND ((state='present' AND absence_reason IS NULL) OR (state='absent' AND absence_reason IN ('unknown','illness','excused')))
    AND ((origin='manual' AND change_reason IS NULL)
      OR (origin='close_default' AND state='absent' AND absence_reason='unknown' AND change_reason IS NULL)
      OR (origin='correction' AND char_length(btrim(change_reason)) BETWEEN 1 AND 1000)))
) IS TRUE);
CREATE OR REPLACE FUNCTION pride.guard_attendance() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s pride.training_sessions; record_date date;
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'attendance snapshot cannot be deleted' USING ERRCODE='23514';
    END IF;
    SELECT * INTO s FROM pride.training_sessions WHERE id=NEW.training_session_id FOR UPDATE;
    IF s.superseded_at IS NOT NULL OR s.status NOT IN ('planned','open','closed') THEN
        RAISE EXCEPTION 'attendance session is not writable' USING ERRCODE='23514';
    END IF;
    IF TG_OP='INSERT' THEN
        IF s.status<>'planned' OR s.roster_frozen_at IS NOT NULL OR NEW.state IS NOT NULL THEN
            RAISE EXCEPTION 'attendance snapshot is already frozen' USING ERRCODE='23514';
        END IF;
        record_date := (s.starts_at AT TIME ZONE s.timezone)::date;
        IF NOT EXISTS(SELECT 1 FROM pride.group_memberships m WHERE m.id=NEW.source_group_membership_id
          AND m.valid_from<=record_date AND (m.valid_until IS NULL OR record_date<m.valid_until)) THEN
            RAISE EXCEPTION 'snapshot source is outside session date' USING ERRCODE='23514';
        END IF;
    ELSE
        IF (NEW.id,NEW.club_id,NEW.training_session_id,NEW.group_id,NEW.club_membership_id,NEW.source_group_membership_id,NEW.created_at)
          IS DISTINCT FROM (OLD.id,OLD.club_id,OLD.training_session_id,OLD.group_id,OLD.club_membership_id,OLD.source_group_membership_id,OLD.created_at) THEN
            RAISE EXCEPTION 'attendance snapshot keys are immutable' USING ERRCODE='23514';
        END IF;
        IF NOT ((s.status='open' AND NEW.origin IN ('manual','close_default') AND NEW.change_reason IS NULL)
          OR (s.status='closed' AND NEW.origin='correction' AND NEW.state IS NOT NULL
              AND NEW.version=OLD.version+1 AND char_length(btrim(NEW.change_reason)) BETWEEN 1 AND 1000)) THEN
            RAISE EXCEPTION 'attendance update requires open journal or closed correction' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
