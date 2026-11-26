-- Recorded family stories and the tree people in them (architecture §3, §9.6). The link to the video joins in Phase 5.
-- A short lock timeout, as the foreign keys lock users and persons (§6.2).
BEGIN;
SET LOCAL lock_timeout = '5s';
CREATE TABLE stories (
  id                    UUID PRIMARY KEY DEFAULT uuidv7(),
  title                 TEXT NOT NULL CHECK (btrim(title) <> ''),
  teller_person_id      UUID REFERENCES persons(id) ON DELETE SET NULL,
  status                TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  consent_confirmed_at  TIMESTAMPTZ,
  recorded_on           DATE,
  recorded_on_precision TEXT CHECK (recorded_on_precision IN ('day','month','year','circa')),
  place                 TEXT,
  summary               TEXT,
  language              TEXT CHECK (language IN ('ht','fr','en','mixed')),
  chapters              JSONB NOT NULL DEFAULT '[]',
  transcript            JSONB NOT NULL DEFAULT '[]',
  topics                TEXT[] NOT NULL DEFAULT '{}',
  other_people          TEXT[] NOT NULL DEFAULT '{}',
  -- The jsonb form reads only the lines' strings, unescaped; 'simple' because the stories mix Creole, French and English.
  search_tsv            TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple', title || ' ' || coalesce(summary, ''))
                          || to_tsvector('simple', transcript)) STORED,
  featured_at           TIMESTAMPTZ,
  created_by            UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at            TIMESTAMPTZ,
  CHECK ((recorded_on IS NULL) = (recorded_on_precision IS NULL)),
  CHECK (status <> 'published' OR consent_confirmed_at IS NOT NULL),
  CHECK (featured_at IS NULL OR status = 'published')
);
CREATE INDEX idx_stories_search ON stories USING gin (search_tsv) WHERE deleted_at IS NULL;
CREATE INDEX idx_stories_teller ON stories(teller_person_id) WHERE deleted_at IS NULL;
-- One story of the week at most; a deleted story stops being featured.
CREATE UNIQUE INDEX uq_stories_featured ON stories((true)) WHERE featured_at IS NOT NULL;

CREATE TABLE story_persons (
  story_id  UUID NOT NULL REFERENCES stories(id)  ON DELETE CASCADE,
  person_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  PRIMARY KEY (story_id, person_id)
);
CREATE INDEX idx_story_persons_person ON story_persons(person_id);
COMMIT;
