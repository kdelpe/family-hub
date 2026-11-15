-- Only parent links and couples are stored; siblings, step-parents and the rest are derived (§3.4).
-- parent_child: person_a is the parent. couple: stored once, with person_a < person_b.
CREATE TABLE relationships (
  id         UUID PRIMARY KEY DEFAULT uuidv7(),
  type       TEXT NOT NULL CHECK (type IN ('parent_child','couple')),
  subtype    TEXT NOT NULL,
  ended      BOOLEAN NOT NULL DEFAULT false,
  person_a   UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  person_b   UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  notes      TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT rel_distinct  CHECK (person_a <> person_b),
  CONSTRAINT rel_canonical CHECK (type = 'parent_child' OR person_a < person_b),
  CONSTRAINT rel_subtype   CHECK (
       (type = 'parent_child' AND subtype IN ('biological','adoptive','foster'))
    OR (type = 'couple'       AND subtype IN ('married','partner'))),
  CONSTRAINT rel_ended     CHECK (NOT ended OR type = 'couple'),
  CONSTRAINT rel_unique    UNIQUE (type, person_a, person_b)
);
CREATE INDEX idx_rel_a ON relationships(person_a);
CREATE INDEX idx_rel_b ON relationships(person_b);
