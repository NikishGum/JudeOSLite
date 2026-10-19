-- RD-12: an inbox row is delivered in the source transaction, never before commit.
-- No historical backfill: installing the module does not notify about old events.
CREATE TABLE pride.notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES pride.staff_users(id),
    club_id uuid NOT NULL REFERENCES pride.clubs(id),
    family_id uuid,
    membership_id uuid,
    session_id uuid,
    event_key text NOT NULL CHECK(char_length(event_key) BETWEEN 1 AND 250),
    kind text NOT NULL CHECK(kind IN ('family','payment','member_document','training_session')),
    title text NOT NULL CHECK(char_length(title) BETWEEN 1 AND 200),
    body text NOT NULL CHECK(char_length(body)<=500),
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id,event_key),
    FOREIGN KEY(family_id,club_id) REFERENCES pride.families(id,club_id),
    FOREIGN KEY(membership_id,club_id) REFERENCES pride.club_memberships(id,club_id),
    FOREIGN KEY(session_id,club_id) REFERENCES pride.training_sessions(id,club_id),
    CHECK((kind='training_session' AND session_id IS NOT NULL AND family_id IS NULL AND membership_id IS NULL)
       OR (kind='family' AND family_id IS NOT NULL AND membership_id IS NULL AND session_id IS NULL)
       OR (kind IN ('payment','member_document') AND membership_id IS NOT NULL AND session_id IS NULL))
);
CREATE INDEX notifications_inbox ON pride.notifications(user_id,created_at DESC,id DESC);
CREATE INDEX notifications_unread ON pride.notifications(user_id,created_at DESC,id DESC) WHERE read_at IS NULL;

CREATE FUNCTION pride.deliver_notification(event text, club uuid, family uuid, member uuid,
    session uuid, kind text, title text, body text) RETURNS void LANGUAGE sql AS $$
    INSERT INTO pride.notifications(user_id,club_id,family_id,membership_id,session_id,event_key,kind,title,body)
    SELECT u.id,club,family,member,session,event,kind,title,body
    FROM pride.staff_users u WHERE u.disabled_at IS NULL AND (
        u.is_owner OR EXISTS(SELECT 1 FROM pride.staff_club_roles r
            WHERE r.staff_user_id=u.id AND r.club_id=club AND r.role='administrator')
        OR (family IS NOT NULL AND EXISTS(SELECT 1 FROM pride.family_access a
            JOIN pride.clubs c ON c.id=club AND c.archived_at IS NULL
            WHERE a.user_id=u.id AND a.family_id=family))
        OR (session IS NOT NULL AND EXISTS(SELECT 1 FROM pride.training_sessions t
            JOIN pride.staff_club_roles r ON r.club_id=t.club_id AND r.staff_user_id=u.id AND r.role='trainer'
            JOIN pride.clubs c ON c.id=t.club_id AND c.archived_at IS NULL
            WHERE t.id=session AND t.trainer_id=u.id AND t.superseded_at IS NULL)))
    ON CONFLICT(user_id,event_key) DO NOTHING;
$$;

-- Existing audit writes already follow each successful family/payment/document
-- mutation. Their version/action is the stable event identity even on replay.
CREATE FUNCTION pride.notify_domain_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE family uuid; member uuid; label text; event text;
BEGIN
    IF NEW.entity_type='family' AND NEW.action IN
        ('family_created','family_updated','family_account_created','family_password_reset','family_access_revoked') THEN
        family:=NEW.entity_id;
        label:=CASE NEW.action WHEN 'family_account_created' THEN 'Создан аккаунт родителя'
            WHEN 'family_password_reset' THEN 'Пароль родителя изменён'
            WHEN 'family_access_revoked' THEN 'Доступ к семье изменён'
            WHEN 'family_created' THEN 'Создана семья' ELSE 'Обновлена карточка семьи' END;
    ELSIF NEW.entity_type='payment' AND NEW.action IN ('payment_created','payment_paid','payment_voided') THEN
        SELECT membership_id INTO member FROM pride.payments WHERE id=NEW.entity_id AND club_id=NEW.club_id;
        label:=CASE NEW.action WHEN 'payment_paid' THEN 'Оплата подтверждена'
            WHEN 'payment_voided' THEN 'Платёж аннулирован' ELSE 'Выставлен новый счёт' END;
    ELSIF NEW.entity_type='member_document' AND NEW.action IN ('member_document_uploaded','member_document_deleted') THEN
        member:=NULLIF(NEW.after_state->>'membershipId','')::uuid;
        label:=CASE NEW.action WHEN 'member_document_uploaded' THEN 'Добавлен документ участника'
            ELSE 'Удалён документ участника' END;
    ELSE RETURN NEW;
    END IF;
    IF member IS NOT NULL THEN
        SELECT family_id INTO family FROM pride.family_children WHERE membership_id=member AND club_id=NEW.club_id;
    END IF;
    event:=NEW.entity_type||':'||NEW.entity_id||':'||NEW.action||':'||coalesce(NEW.after_state->>'version',NEW.id::text);
    PERFORM pride.deliver_notification(event,NEW.club_id,family,member,NULL,NEW.entity_type,label,
        'Откройте раздел, чтобы посмотреть изменения.');
    RETURN NEW;
END;
$$;
CREATE TRIGGER notify_domain_event AFTER INSERT ON pride.audit_events
    FOR EACH ROW EXECUTE FUNCTION pride.notify_domain_event();

-- A row trigger also covers the hourly generator, which intentionally has no
-- human audit actor. Journal open/mark/close/correction do not send schedule news.
CREATE FUNCTION pride.notify_training_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE label text;
BEGIN
    IF TG_OP='INSERT' THEN
        label:='Назначено занятие';
    ELSIF NEW.superseded_at IS DISTINCT FROM OLD.superseded_at THEN
        label:='Расписание обновлено';
    ELSIF NEW.status='cancelled' AND OLD.status<>'cancelled' THEN
        label:='Занятие отменено';
    ELSIF NEW.trainer_id IS DISTINCT FROM OLD.trainer_id THEN
        label:='Заменён тренер занятия';
    ELSIF (NEW.starts_at,NEW.ends_at) IS DISTINCT FROM (OLD.starts_at,OLD.ends_at) THEN
        label:='Занятие перенесено';
    ELSE RETURN NEW;
    END IF;
    PERFORM pride.deliver_notification('training_session:'||NEW.id||':'||NEW.version,
        NEW.club_id,NULL,NULL,NEW.id,'training_session',label,
        to_char(NEW.starts_at AT TIME ZONE NEW.timezone,'DD.MM.YYYY HH24:MI'));
    RETURN NEW;
END;
$$;
CREATE TRIGGER notify_training_event AFTER INSERT OR UPDATE ON pride.training_sessions
    FOR EACH ROW EXECUTE FUNCTION pride.notify_training_event();
