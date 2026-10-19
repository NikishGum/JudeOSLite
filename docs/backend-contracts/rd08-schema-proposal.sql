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

CREATE TABLE pride.billing_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL REFERENCES pride.clubs(id),
    name text NOT NULL CHECK(char_length(btrim(name)) BETWEEN 1 AND 200),
    amount_minor bigint NOT NULL CHECK(amount_minor BETWEEN 1 AND 999999999),
    visits integer NOT NULL CHECK(visits BETWEEN 1 AND 366),
    days integer NOT NULL CHECK(days BETWEEN 1 AND 366),
    archived boolean NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1 CHECK(version>0),
    UNIQUE(id,club_id)
);
CREATE TABLE pride.payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL,
    membership_id uuid NOT NULL,
    request_id uuid NOT NULL UNIQUE,
    description text NOT NULL CHECK(char_length(btrim(description)) BETWEEN 1 AND 300),
    amount_minor bigint NOT NULL CHECK(amount_minor BETWEEN 1 AND 999999999),
    currency text NOT NULL DEFAULT 'BYN' CHECK(currency='BYN'),
    method text CHECK(method IN ('cash','card','transfer','erip')),
    status text NOT NULL CHECK(status IN ('pending','paid','void')),
    paid_on date CHECK(isfinite(paid_on)),
    void_reason text CHECK(char_length(btrim(void_reason)) BETWEEN 1 AND 500),
    version integer NOT NULL DEFAULT 1 CHECK(version>0),
    created_by uuid NOT NULL REFERENCES pride.staff_users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY(membership_id,club_id) REFERENCES pride.club_memberships(id,club_id),
    UNIQUE(id,club_id),
    CHECK((status='pending' AND paid_on IS NULL AND method IS NULL AND void_reason IS NULL)
      OR (status='paid' AND paid_on IS NOT NULL AND method IS NOT NULL AND void_reason IS NULL)
      OR (status='void' AND void_reason IS NOT NULL))
);
CREATE INDEX payments_club_date ON pride.payments(club_id,created_at DESC,id);
CREATE INDEX payments_membership ON pride.payments(membership_id);
CREATE TABLE pride.subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL,
    membership_id uuid NOT NULL,
    payment_id uuid NOT NULL UNIQUE,
    plan_name text NOT NULL,
    visits integer NOT NULL CHECK(visits BETWEEN 1 AND 366),
    valid_from date NOT NULL CHECK(isfinite(valid_from)),
    valid_until date NOT NULL CHECK(isfinite(valid_until) AND valid_until>valid_from),
    cancelled_at timestamptz,
    FOREIGN KEY(payment_id,club_id) REFERENCES pride.payments(id,club_id),
    FOREIGN KEY(membership_id,club_id) REFERENCES pride.club_memberships(id,club_id)
);
CREATE INDEX subscriptions_member ON pride.subscriptions(membership_id);
CREATE FUNCTION pride.guard_subscription() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM id FROM pride.club_memberships WHERE id=NEW.membership_id FOR UPDATE;
    IF NEW.cancelled_at IS NULL AND EXISTS(SELECT 1 FROM pride.subscriptions s WHERE s.membership_id=NEW.membership_id AND s.id<>NEW.id AND s.cancelled_at IS NULL AND daterange(s.valid_from,s.valid_until,'[)') && daterange(NEW.valid_from,NEW.valid_until,'[)')) THEN
        RAISE EXCEPTION 'overlapping subscriptions' USING ERRCODE='23P01';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER guard_subscription BEFORE INSERT OR UPDATE ON pride.subscriptions FOR EACH ROW EXECUTE FUNCTION pride.guard_subscription();

CREATE TABLE pride.member_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL,
    membership_id uuid NOT NULL,
    name text NOT NULL CHECK(char_length(btrim(name)) BETWEEN 1 AND 200),
    kind text NOT NULL CHECK(kind IN ('medical','contract','consent','other')),
    filename text NOT NULL CHECK(char_length(filename) BETWEEN 1 AND 200),
    media_type text NOT NULL CHECK(media_type IN ('application/pdf','image/png','image/jpeg')),
    content bytea NOT NULL CHECK(octet_length(content) BETWEEN 1 AND 5242880),
    expires_on date CHECK(isfinite(expires_on)),
    version integer NOT NULL DEFAULT 1 CHECK(version>0),
    created_by uuid NOT NULL REFERENCES pride.staff_users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY(membership_id,club_id) REFERENCES pride.club_memberships(id,club_id)
);
CREATE INDEX member_documents_member ON pride.member_documents(membership_id);

CREATE TABLE pride.notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES pride.staff_users(id),
    club_id uuid REFERENCES pride.clubs(id),
    family_id uuid REFERENCES pride.families(id),
    session_id uuid REFERENCES pride.training_sessions(id),
    event_id uuid NOT NULL REFERENCES pride.audit_events(id),
    kind text NOT NULL,
    title text NOT NULL,
    href text NOT NULL,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id,event_id)
);
CREATE INDEX notifications_inbox ON pride.notifications(user_id,created_at DESC,id);
CREATE FUNCTION pride.notify_club_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE family uuid; member uuid; session uuid; label text; target text;
BEGIN
    IF NEW.entity_type='payment' THEN
        SELECT membership_id INTO member FROM pride.payments WHERE id=NEW.entity_id;
        label:=CASE NEW.action WHEN 'payment_paid' THEN 'Оплата подтверждена' WHEN 'payment_voided' THEN 'Платёж аннулирован' ELSE 'Выставлен новый счёт' END;
        target:='/payments';
    ELSIF NEW.entity_type='member_document' THEN
        member:=NULLIF(NEW.after_state->>'membershipId','')::uuid;
        label:='Обновлены документы участника'; target:='/families';
    ELSIF NEW.entity_type='family' THEN
        family:=NEW.entity_id; label:='Обновлена карточка семьи'; target:='/families';
    ELSIF NEW.entity_type='training_session' THEN
        session:=NEW.entity_id; label:='Изменено назначенное занятие'; target:='/sessions?session='||session;
    ELSE RETURN NEW;
    END IF;
    IF member IS NOT NULL THEN SELECT family_id INTO family FROM pride.family_children WHERE membership_id=member; END IF;
    INSERT INTO pride.notifications(user_id,club_id,family_id,session_id,event_id,kind,title,href)
    SELECT u.id,NEW.club_id,family,session,NEW.id,NEW.entity_type,label,'/clubs/'||NEW.club_id||target
    FROM pride.staff_users u WHERE u.disabled_at IS NULL AND u.id<>NEW.actor_id AND (
       u.is_owner OR EXISTS(SELECT 1 FROM pride.staff_club_roles r WHERE r.staff_user_id=u.id AND r.club_id=NEW.club_id AND r.role='administrator')
       OR (family IS NOT NULL AND EXISTS(SELECT 1 FROM pride.family_access a WHERE a.user_id=u.id AND a.family_id=family))
       OR (session IS NOT NULL AND EXISTS(SELECT 1 FROM pride.training_sessions t JOIN pride.staff_club_roles r ON r.club_id=t.club_id AND r.role='trainer' AND r.staff_user_id=u.id WHERE t.id=session AND t.trainer_id=u.id)))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;
CREATE TRIGGER notify_club_event AFTER INSERT ON pride.audit_events FOR EACH ROW EXECUTE FUNCTION pride.notify_club_event();
