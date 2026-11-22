BEGIN;
SET LOCAL lock_timeout = '5s';
DROP TABLE event_persons, events;
COMMIT;
