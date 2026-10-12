CREATE TABLE pride.schedule_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL REFERENCES pride.clubs(id) ON DELETE RESTRICT,
    group_id uuid NOT NULL,
    series_id uuid NOT NULL DEFAULT gen_random_uuid(),
    trainer_id uuid NOT NULL REFERENCES pride.staff_users(id) ON DELETE RESTRICT,
    mode text NOT NULL CHECK (mode IN ('weekly','day_parity','date_list')),
    weekdays integer[],
    day_parity text,
    selected_dates date[],
    starts_time time NOT NULL CHECK (starts_time < TIME '24:00'),
    duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 1 AND 1440),
    timezone text NOT NULL,
    valid_from date NOT NULL CHECK (isfinite(valid_from) AND valid_from >= DATE '0001-01-01'),
    valid_until date CHECK (isfinite(valid_until) AND valid_until > valid_from),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (group_id,club_id) REFERENCES pride.training_groups(id,club_id) ON DELETE RESTRICT,
    UNIQUE(id,club_id,group_id),
    UNIQUE(series_id,valid_from),
    CHECK (((mode='weekly' AND cardinality(weekdays) BETWEEN 1 AND 7 AND weekdays <@ ARRAY[1,2,3,4,5,6,7] AND array_position(weekdays,NULL) IS NULL AND day_parity IS NULL AND selected_dates IS NULL)
        OR (mode='day_parity' AND weekdays IS NULL AND day_parity IN ('even','odd') AND selected_dates IS NULL)
        OR (mode='date_list' AND weekdays IS NULL AND day_parity IS NULL AND cardinality(selected_dates) BETWEEN 1 AND 1000 AND array_position(selected_dates,NULL) IS NULL)) IS TRUE)
);
CREATE INDEX schedule_rules_club_group_dates ON pride.schedule_rules(club_id,group_id,valid_from);

CREATE TABLE pride.training_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL REFERENCES pride.clubs(id) ON DELETE RESTRICT,
    group_id uuid NOT NULL,
    schedule_rule_id uuid,
    trainer_id uuid NOT NULL REFERENCES pride.staff_users(id) ON DELETE RESTRICT,
    starts_on date NOT NULL CHECK (isfinite(starts_on)),
    timezone text NOT NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL CHECK (ends_at > starts_at),
    status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','open','closed','cancelled')),
    is_manual_override boolean NOT NULL DEFAULT false,
    roster_frozen_at timestamptz,
    closed_at timestamptz,
    cancelled_at timestamptz,
    cancellation_reason text,
    superseded_at timestamptz,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (group_id,club_id) REFERENCES pride.training_groups(id,club_id) ON DELETE RESTRICT,
    FOREIGN KEY (schedule_rule_id,club_id,group_id) REFERENCES pride.schedule_rules(id,club_id,group_id) ON DELETE RESTRICT,
    UNIQUE(id,club_id),
    UNIQUE(id,club_id,group_id),
    CHECK (superseded_at IS NULL OR (status='planned' AND NOT is_manual_override AND roster_frozen_at IS NULL))
);
-- Cancelled sessions still reserve the original slot. Retired ordinary future
-- plans retain their IDs/history but no longer occupy a live calendar slot.
CREATE UNIQUE INDEX training_sessions_group_start ON pride.training_sessions(group_id,starts_at) WHERE superseded_at IS NULL;
CREATE INDEX training_sessions_club_start ON pride.training_sessions(club_id,starts_at);
CREATE INDEX training_sessions_trainer_start ON pride.training_sessions(trainer_id,starts_at);
CREATE INDEX training_sessions_rule ON pride.training_sessions(schedule_rule_id);

CREATE FUNCTION pride.guard_schedule_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(155007);
    IF NEW.mode='date_list' AND EXISTS (SELECT 1 FROM unnest(NEW.selected_dates) d WHERE NOT isfinite(d) OR d<NEW.valid_from) THEN
        RAISE EXCEPTION 'invalid selected date' USING ERRCODE='23514';
    END IF;
    IF TG_OP='UPDATE' AND (NEW.id,NEW.club_id,NEW.group_id,NEW.series_id,NEW.trainer_id,NEW.mode,NEW.weekdays,NEW.day_parity,NEW.selected_dates,NEW.starts_time,NEW.duration_minutes,NEW.timezone,NEW.valid_from)
        IS DISTINCT FROM (OLD.id,OLD.club_id,OLD.group_id,OLD.series_id,OLD.trainer_id,OLD.mode,OLD.weekdays,OLD.day_parity,OLD.selected_dates,OLD.starts_time,OLD.duration_minutes,OLD.timezone,OLD.valid_from) THEN
        RAISE EXCEPTION 'rule content is immutable; create dated version' USING ERRCODE='23514';
    END IF;
    IF EXISTS (SELECT 1 FROM pride.schedule_rules r WHERE r.series_id=NEW.series_id AND r.id<>NEW.id
        AND (r.club_id<>NEW.club_id OR r.group_id<>NEW.group_id OR daterange(r.valid_from,r.valid_until,'[)') && daterange(NEW.valid_from,NEW.valid_until,'[)'))) THEN
        RAISE EXCEPTION 'overlapping schedule versions' USING ERRCODE='23P01';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER guard_schedule_version BEFORE INSERT OR UPDATE ON pride.schedule_rules FOR EACH ROW EXECUTE FUNCTION pride.guard_schedule_version();
