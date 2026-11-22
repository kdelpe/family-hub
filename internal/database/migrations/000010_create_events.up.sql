-- Events on the family timeline and who was there (architecture §3). Photos and videos join them in Phase 5.
-- A short lock timeout, as the foreign keys lock users and persons (§6.2).
BEGIN;
SET LOCAL lock_timeout = '5s';
CREATE TABLE events (
  id                   UUID PRIMARY KEY DEFAULT uuidv7(),
  title                TEXT NOT NULL CHECK (btrim(title) <> ''),
  event_date           DATE,
  event_date_precision TEXT CHECK (event_date_precision IN ('day','month','year','circa')),
  location             TEXT,
  description          TEXT,
  created_by           UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at           TIMESTAMPTZ,
  CHECK ((event_date IS NULL) = (event_date_precision IS NULL))
);
CREATE INDEX idx_events_date ON events(event_date DESC NULLS LAST) WHERE deleted_at IS NULL;

CREATE TABLE event_persons (
  event_id  UUID NOT NULL REFERENCES events(id)  ON DELETE CASCADE,
  person_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  PRIMARY KEY (event_id, person_id)
);
CREATE INDEX idx_event_persons_person ON event_persons(person_id);
COMMIT;
