-- Who each login is in the tree (§5.1). Persons are only soft-deleted, so SET NULL is for a manual purge.
ALTER TABLE users ADD COLUMN person_id UUID CONSTRAINT fk_users_person REFERENCES persons(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX uq_users_person ON users(person_id) WHERE person_id IS NOT NULL;
