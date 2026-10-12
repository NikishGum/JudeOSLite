-- Minimal club identity is needed by access assignments; management follows in V1-03.
CREATE TABLE pride.clubs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    address text,
    timezone text NOT NULL DEFAULT 'Europe/Minsk',
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pride.staff_users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login text NOT NULL UNIQUE CHECK (login = lower(btrim(login)) AND login ~ '^[a-z0-9][a-z0-9._@+\-]{0,99}$'),
    display_name text NOT NULL CHECK (length(btrim(display_name)) BETWEEN 1 AND 200),
    password_hash text NOT NULL,
    is_owner boolean NOT NULL DEFAULT false,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE pride.staff_club_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_user_id uuid NOT NULL REFERENCES pride.staff_users(id) ON DELETE RESTRICT,
    club_id uuid NOT NULL REFERENCES pride.clubs(id) ON DELETE RESTRICT,
    role text NOT NULL CHECK (role IN ('administrator', 'trainer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (staff_user_id, club_id, role)
);
CREATE TABLE pride.staff_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_user_id uuid NOT NULL REFERENCES pride.staff_users(id) ON DELETE RESTRICT,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_token_hash bytea NOT NULL CHECK (octet_length(csrf_token_hash) = 32),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX staff_sessions_expires ON pride.staff_sessions(expires_at);
CREATE INDEX staff_sessions_user ON pride.staff_sessions(staff_user_id);
-- Disabling an account permanently revokes existing sessions, even after re-enabling.
CREATE FUNCTION pride.revoke_disabled_staff_sessions() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.disabled_at IS NOT NULL AND OLD.disabled_at IS NULL THEN
        UPDATE pride.staff_sessions SET revoked_at = now()
        WHERE staff_user_id = NEW.id AND revoked_at IS NULL;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER staff_disabled_sessions AFTER UPDATE OF disabled_at ON pride.staff_users
FOR EACH ROW EXECUTE FUNCTION pride.revoke_disabled_staff_sessions();
