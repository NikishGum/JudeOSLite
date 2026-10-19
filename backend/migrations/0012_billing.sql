-- RD-10: exact manual finances; no gateway or legacy debt import.
CREATE EXTENSION IF NOT EXISTS btree_gist;

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
    request_body jsonb NOT NULL CHECK(jsonb_typeof(request_body)='object'),
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
    UNIQUE(id,club_id,membership_id),
    CHECK((method IS NULL)=(paid_on IS NULL)),
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
    plan_name text NOT NULL CHECK(char_length(btrim(plan_name)) BETWEEN 1 AND 200),
    visits integer NOT NULL CHECK(visits BETWEEN 1 AND 366),
    days integer NOT NULL CHECK(days BETWEEN 1 AND 366),
    valid_from date NOT NULL CHECK(isfinite(valid_from)),
    valid_until date NOT NULL CHECK(isfinite(valid_until) AND valid_until>valid_from),
    cancelled_at timestamptz,
    CHECK(valid_until=valid_from+days),
    EXCLUDE USING gist (membership_id WITH =, daterange(valid_from,valid_until,'[)') WITH &&) WHERE (cancelled_at IS NULL),
    FOREIGN KEY(payment_id,club_id,membership_id) REFERENCES pride.payments(id,club_id,membership_id),
    FOREIGN KEY(membership_id,club_id) REFERENCES pride.club_memberships(id,club_id)
);
CREATE INDEX subscriptions_member ON pride.subscriptions(membership_id);
CREATE INDEX subscriptions_club ON pride.subscriptions(club_id,id);
CREATE INDEX billing_plans_club_name ON pride.billing_plans(club_id,lower(name),id);
CREATE INDEX payments_club_paid ON pride.payments(club_id,paid_on,id) WHERE status='paid';
