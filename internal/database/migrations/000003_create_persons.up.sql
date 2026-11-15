CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE persons (
  id                   UUID PRIMARY KEY DEFAULT uuidv7(),
  first_name           TEXT NOT NULL,
  middle_name          TEXT,
  last_name            TEXT,
  maiden_name          TEXT,
  side                 TEXT CHECK (side IN ('first','second','both','married_in')),
  birth_place          TEXT,
  gender               TEXT CHECK (gender IN ('male','female','other','unknown')),
  birth_date           DATE,
  birth_date_precision TEXT CHECK (birth_date_precision IN ('day','month','year','circa')),
  death_date           DATE,
  death_date_precision TEXT CHECK (death_date_precision IN ('day','month','year','circa')),
  is_deceased          BOOLEAN NOT NULL DEFAULT false,
  bio                  TEXT,
  -- concat_ws isn't immutable, so it can't be used in a generated column.
  search_name          TEXT GENERATED ALWAYS AS (first_name || coalesce(' ' || last_name, '')) STORED,
  created_by           UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at           TIMESTAMPTZ,
  CHECK ((birth_date IS NULL) = (birth_date_precision IS NULL)),
  CHECK ((death_date IS NULL) = (death_date_precision IS NULL)),
  CHECK (death_date IS NULL OR is_deceased),
  CHECK (btrim(first_name) <> ''),
  CHECK (last_name IS NULL OR btrim(last_name) <> '')
);
CREATE INDEX idx_persons_name_trgm ON persons USING gin (search_name gin_trgm_ops) WHERE deleted_at IS NULL;

-- Every query that returns people to a browser takes the birth date from here, never birth_date (§5.2, §10).
-- "--MM-DD" means the year is hidden.
CREATE FUNCTION masked_birth_date(p persons, viewer_is_admin boolean, viewer_person_id uuid)
RETURNS text LANGUAGE sql STABLE AS $$
  SELECT CASE
    WHEN p.birth_date IS NULL THEN NULL
    WHEN viewer_is_admin OR p.is_deceased OR p.id = viewer_person_id THEN
      CASE p.birth_date_precision
        WHEN 'day'   THEN to_char(p.birth_date, 'YYYY-MM-DD')
        WHEN 'month' THEN to_char(p.birth_date, 'YYYY-MM')
        ELSE              to_char(p.birth_date, 'YYYY')
      END
    WHEN p.birth_date_precision = 'day'   THEN to_char(p.birth_date, '--MM-DD')
    WHEN p.birth_date_precision = 'month' THEN to_char(p.birth_date, '--MM')
    ELSE NULL
  END
$$;
