-- Someone asks for an invitation, or a member suggests a relative; a request never creates an account (§5).
-- A short lock timeout, as the foreign keys lock users (§6.2).
BEGIN;
SET LOCAL lock_timeout = '5s';
CREATE TABLE access_requests (
  id              UUID PRIMARY KEY DEFAULT uuidv7(),
  full_name       TEXT NOT NULL CHECK (btrim(full_name) <> ''),
  email           TEXT NOT NULL CHECK (email = lower(email)),
  side            TEXT CHECK (side IN ('first','second','married_in')),
  relationship    TEXT,
  related_names   TEXT[] NOT NULL DEFAULT '{}',
  note            TEXT,
  source          TEXT NOT NULL CHECK (source IN ('self','member_suggestion')),
  suggested_by    UUID REFERENCES users(id) ON DELETE SET NULL,
  status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','declined')),
  reviewed_by     UUID REFERENCES users(id) ON DELETE SET NULL,
  reviewed_at     TIMESTAMPTZ,
  invited_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- Not a CHECK pairing source with suggested_by: ON DELETE SET NULL would then fail when the suggester goes.
  CHECK (source = 'member_suggestion' OR suggested_by IS NULL)
);
CREATE INDEX idx_access_requests_pending ON access_requests(created_at) WHERE status = 'pending';
CREATE UNIQUE INDEX uq_access_requests_pending_email ON access_requests(email) WHERE status = 'pending';
CREATE INDEX idx_access_requests_declined ON access_requests(reviewed_at) WHERE status = 'declined';
COMMIT;
