-- Private binary documents; only authenticated domain handlers serve content.
CREATE TABLE pride.member_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id uuid NOT NULL,
    membership_id uuid NOT NULL,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    kind text NOT NULL CHECK (kind IN ('medical','contract','consent','other')),
    filename text NOT NULL CHECK (char_length(filename) BETWEEN 1 AND 200 AND octet_length(filename)<=255 AND strpos(filename,'/')=0 AND strpos(filename,chr(92))=0 AND strpos(filename,chr(13))=0 AND strpos(filename,chr(10))=0),
    media_type text NOT NULL CHECK (media_type IN ('application/pdf','image/png','image/jpeg')),
    content bytea NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 5242880),
    size integer GENERATED ALWAYS AS (octet_length(content)) STORED,
    expires_on date CHECK (isfinite(expires_on) AND expires_on BETWEEN DATE '0001-01-01' AND DATE '9999-12-31'),
    version integer NOT NULL DEFAULT 1 CHECK (version>0),
    created_by uuid NOT NULL REFERENCES pride.staff_users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (membership_id,club_id) REFERENCES pride.club_memberships(id,club_id)
);
CREATE INDEX member_documents_member_created ON pride.member_documents(membership_id,created_at DESC,id DESC);
