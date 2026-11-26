BEGIN;
SET LOCAL lock_timeout = '5s';
DROP TABLE story_persons, stories;
COMMIT;
