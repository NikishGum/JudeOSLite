-- Extend the existing V1-02 tables; preserve IDs, assignments and sessions.
ALTER TABLE pride.clubs ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK (version > 0);
ALTER TABLE pride.staff_users ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE TABLE pride.audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid REFERENCES pride.clubs(id) ON DELETE RESTRICT,
    actor_id uuid NOT NULL REFERENCES pride.staff_users(id) ON DELETE RESTRICT,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    action text NOT NULL,
    before_state jsonb,
    after_state jsonb,
    change_reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_entity ON pride.audit_events(entity_type, entity_id, created_at);
