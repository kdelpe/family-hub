DROP FUNCTION masked_birth_date(persons, boolean, uuid);
DROP TABLE persons;
-- pg_trgm stays: an extension is database-wide, and other schemas (tests run side by side) may use it.
