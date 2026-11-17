-- Contact details, hidden by default; a row's visibility decides who else sees it (§5.2).
-- A short lock timeout, as the foreign key locks persons (§6.2).
BEGIN;
SET LOCAL lock_timeout = '5s';
CREATE TABLE contact_methods (
  id         UUID PRIMARY KEY DEFAULT uuidv7(),
  person_id  UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL CHECK (kind IN ('email','phone','address','social','other')),
  platform   TEXT CHECK (platform IN ('whatsapp','instagram','facebook','tiktok','linkedin','x','youtube','other')),
  label      TEXT,
  value      TEXT NOT NULL,
  visibility TEXT NOT NULL DEFAULT 'hidden' CHECK (visibility IN ('hidden','family','on_request')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((kind = 'social') = (platform IS NOT NULL))
);
CREATE INDEX idx_contact_methods_person ON contact_methods(person_id);
COMMIT;
