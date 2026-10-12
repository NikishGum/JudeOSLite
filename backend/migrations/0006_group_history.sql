CREATE TABLE pride.training_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL REFERENCES pride.clubs(id) ON DELETE RESTRICT,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    archived_at timestamptz,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, club_id)
);
CREATE INDEX training_groups_club_archive ON pride.training_groups(club_id, archived_at);
CREATE TABLE pride.group_memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL REFERENCES pride.clubs(id) ON DELETE RESTRICT,
    group_id uuid NOT NULL,
    club_membership_id uuid NOT NULL,
    valid_from date NOT NULL CHECK (isfinite(valid_from) AND valid_from >= DATE '0001-01-01'),
    valid_until date CHECK (isfinite(valid_until) AND valid_until > valid_from),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (group_id, club_id) REFERENCES pride.training_groups(id, club_id) ON DELETE RESTRICT,
    FOREIGN KEY (club_membership_id, club_id) REFERENCES pride.club_memberships(id, club_id) ON DELETE RESTRICT,
    UNIQUE (id, club_id),
    UNIQUE (id, club_id, group_id, club_membership_id)
);
CREATE INDEX group_memberships_group_dates ON pride.group_memberships(group_id, valid_from, valid_until);
CREATE INDEX group_memberships_member_dates ON pride.group_memberships(club_membership_id, valid_from);

-- A parent lock serializes every interval change, including direct SQL. No GiST
-- extension is needed. One club card belongs to at most one group on any date.
CREATE FUNCTION pride.guard_group_interval() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE person uuid; joined date;
BEGIN
    IF TG_OP = 'UPDATE' AND (NEW.id,NEW.club_id,NEW.group_id,NEW.club_membership_id,NEW.valid_from)
        IS DISTINCT FROM (OLD.id,OLD.club_id,OLD.group_id,OLD.club_membership_id,OLD.valid_from) THEN
        RAISE EXCEPTION 'interval keys are immutable' USING ERRCODE='23514';
    END IF;
    SELECT participant_id INTO person FROM pride.club_memberships WHERE id=NEW.club_membership_id AND club_id=NEW.club_id;
    PERFORM id FROM pride.participants WHERE id=person FOR UPDATE;
    SELECT joined_on INTO joined FROM pride.club_memberships WHERE id=NEW.club_membership_id AND club_id=NEW.club_id FOR UPDATE;
    IF NEW.valid_from < joined THEN
        RAISE EXCEPTION 'interval before club entry' USING ERRCODE='23514';
    END IF;
    IF EXISTS (SELECT 1 FROM pride.group_memberships m WHERE m.club_membership_id=NEW.club_membership_id AND m.id<>NEW.id
        AND daterange(m.valid_from,m.valid_until,'[)') && daterange(NEW.valid_from,NEW.valid_until,'[)')) THEN
        RAISE EXCEPTION 'overlapping group intervals' USING ERRCODE='23P01';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER guard_group_interval BEFORE INSERT OR UPDATE ON pride.group_memberships
FOR EACH ROW EXECUTE FUNCTION pride.guard_group_interval();
CREATE FUNCTION pride.guard_group_entry_date() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM pride.group_memberships WHERE club_membership_id=NEW.id AND valid_from<NEW.joined_on) THEN
        RAISE EXCEPTION 'club entry after group entry' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER guard_group_entry_date BEFORE UPDATE OF joined_on ON pride.club_memberships
FOR EACH ROW EXECUTE FUNCTION pride.guard_group_entry_date();
