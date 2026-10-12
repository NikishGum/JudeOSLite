CREATE TABLE pride.participants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name text NOT NULL CHECK (char_length(btrim(full_name)) BETWEEN 1 AND 200),
    birth_date date NOT NULL CHECK (isfinite(birth_date) AND birth_date >= DATE '0001-01-01'),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pride.club_memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    participant_id uuid NOT NULL REFERENCES pride.participants(id) ON DELETE RESTRICT,
    club_id uuid NOT NULL REFERENCES pride.clubs(id) ON DELETE RESTRICT,
    joined_on date NOT NULL CHECK (isfinite(joined_on) AND joined_on >= DATE '0001-01-01'),
    contact_kind text NOT NULL CHECK (contact_kind IN ('phone','email')),
    contact_value text NOT NULL CHECK (char_length(btrim(contact_value)) BETWEEN 1 AND 254),
    contact_name text CHECK (char_length(contact_name) <= 200),
    archived_at timestamptz,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (participant_id, club_id),
    UNIQUE (id, club_id)
);
CREATE INDEX club_memberships_club_archive ON pride.club_memberships(club_id, archived_at);

-- Serialize a new club link with identity edits and their single-club permission
-- check. Archived links still count. No matching/linking endpoint is added in v1.
CREATE FUNCTION pride.lock_membership_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM id FROM pride.participants WHERE id=NEW.participant_id FOR UPDATE;
    RETURN NEW;
END;
$$;
CREATE TRIGGER lock_membership_identity BEFORE INSERT OR UPDATE OF participant_id, club_id
ON pride.club_memberships FOR EACH ROW EXECUTE FUNCTION pride.lock_membership_identity();
