CREATE TABLE pride.attendance_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL,
    training_session_id uuid NOT NULL,
    group_id uuid NOT NULL,
    club_membership_id uuid NOT NULL,
    source_group_membership_id uuid NOT NULL,
    state text,
    absence_reason text,
    marked_by uuid REFERENCES pride.staff_users(id) ON DELETE RESTRICT,
    marked_at timestamptz,
    origin text,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY(training_session_id,club_id,group_id) REFERENCES pride.training_sessions(id,club_id,group_id) ON DELETE RESTRICT,
    FOREIGN KEY(club_membership_id,club_id) REFERENCES pride.club_memberships(id,club_id) ON DELETE RESTRICT,
    FOREIGN KEY(source_group_membership_id,club_id,group_id,club_membership_id) REFERENCES pride.group_memberships(id,club_id,group_id,club_membership_id) ON DELETE RESTRICT,
    UNIQUE(training_session_id,club_membership_id),
    CHECK ((
      (state IS NULL AND absence_reason IS NULL AND marked_by IS NULL AND marked_at IS NULL AND origin IS NULL)
      OR (state='present' AND absence_reason IS NULL AND marked_by IS NOT NULL AND marked_at IS NOT NULL AND origin='manual')
      OR (state='absent' AND absence_reason IN ('unknown','illness','excused') AND marked_by IS NOT NULL AND marked_at IS NOT NULL AND origin IN ('manual','close_default'))
    ) IS TRUE)
);
-- Serialize even direct SQL with the same session row. Frozen snapshot keys and
-- membership rows never disappear; V1-11 will extend updates for corrections.
CREATE FUNCTION pride.guard_attendance() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s pride.training_sessions; record_date date;
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'attendance snapshot cannot be deleted' USING ERRCODE='23514';
    END IF;
    SELECT * INTO s FROM pride.training_sessions WHERE id=NEW.training_session_id FOR UPDATE;
    IF s.superseded_at IS NOT NULL OR s.status NOT IN ('planned','open') THEN
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
        IF s.status<>'open' OR (NEW.id,NEW.club_id,NEW.training_session_id,NEW.group_id,NEW.club_membership_id,NEW.source_group_membership_id,NEW.created_at)
          IS DISTINCT FROM (OLD.id,OLD.club_id,OLD.training_session_id,OLD.group_id,OLD.club_membership_id,OLD.source_group_membership_id,OLD.created_at) THEN
            RAISE EXCEPTION 'attendance snapshot keys are immutable' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER guard_attendance BEFORE INSERT OR UPDATE OR DELETE ON pride.attendance_records
FOR EACH ROW EXECUTE FUNCTION pride.guard_attendance();
