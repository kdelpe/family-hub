-- A member asks to see someone's phone, WhatsApp or address; the owner shares with that one person (§5.2).
-- A short lock timeout, as the foreign keys lock users and persons (§6.2).
BEGIN;
SET LOCAL lock_timeout = '5s';
CREATE TABLE contact_requests (
  id           UUID PRIMARY KEY DEFAULT uuidv7(),
  requester_id UUID NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
  person_id    UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL CHECK (kind IN ('phone','whatsapp','address')),
  status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','shared','declined','revoked')),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at   TIMESTAMPTZ,
  UNIQUE (requester_id, person_id, kind)
);
CREATE INDEX idx_contact_requests_person ON contact_requests(person_id);
COMMIT;
