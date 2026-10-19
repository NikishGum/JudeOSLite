-- RD-08: explicit family access; existing staff authentication remains the single session boundary.
CREATE TABLE pride.families (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL REFERENCES pride.clubs(id),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    contact_name text NOT NULL DEFAULT '' CHECK (char_length(contact_name)<=200),
    phone text NOT NULL DEFAULT '' CHECK (char_length(phone)<=80),
    email text NOT NULL DEFAULT '' CHECK (char_length(email)<=254),
    version integer NOT NULL DEFAULT 1 CHECK(version>0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(id,club_id)
);
CREATE TABLE pride.family_children (
    family_id uuid NOT NULL,
    club_id uuid NOT NULL,
    membership_id uuid NOT NULL UNIQUE,
    PRIMARY KEY(family_id,membership_id),
    FOREIGN KEY(family_id,club_id) REFERENCES pride.families(id,club_id),
    FOREIGN KEY(membership_id,club_id) REFERENCES pride.club_memberships(id,club_id)
);
CREATE TABLE pride.family_access (
    family_id uuid NOT NULL REFERENCES pride.families(id),
    user_id uuid NOT NULL REFERENCES pride.staff_users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(family_id,user_id)
);
CREATE INDEX family_access_user ON pride.family_access(user_id);

CREATE INDEX families_club_name ON pride.families(club_id,lower(name),id);
