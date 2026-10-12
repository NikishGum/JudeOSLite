-- Credential changes revoke every existing session, including direct SQL changes.
CREATE FUNCTION pride.revoke_changed_credentials() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.password_hash IS DISTINCT FROM OLD.password_hash OR NEW.login IS DISTINCT FROM OLD.login THEN
        UPDATE pride.staff_sessions SET revoked_at = now()
        WHERE staff_user_id = NEW.id AND revoked_at IS NULL;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER staff_credentials_sessions AFTER UPDATE OF password_hash, login ON pride.staff_users
FOR EACH ROW EXECUTE FUNCTION pride.revoke_changed_credentials();

-- Serialize new/removed assignments with the single-club account permission check.
-- Writers lock staff_users before roles; role deletion never deletes the staff identity.
CREATE FUNCTION pride.lock_staff_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND (NEW.staff_user_id <> OLD.staff_user_id OR NEW.club_id <> OLD.club_id OR NEW.role <> OLD.role) THEN
        RAISE EXCEPTION 'Assignment keys are immutable; revoke and create instead';
    END IF;
    IF TG_OP = 'DELETE' THEN
        PERFORM 1 FROM pride.staff_users WHERE id = OLD.staff_user_id FOR UPDATE;
        RETURN OLD;
    END IF;
    PERFORM 1 FROM pride.staff_users WHERE id = NEW.staff_user_id FOR UPDATE;
    RETURN NEW;
END;
$$;
CREATE TRIGGER staff_assignment_lock BEFORE INSERT OR UPDATE OR DELETE ON pride.staff_club_roles
FOR EACH ROW EXECUTE FUNCTION pride.lock_staff_assignment();
