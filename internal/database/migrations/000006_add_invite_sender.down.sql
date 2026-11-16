BEGIN;
SET LOCAL lock_timeout = '5s';
ALTER TABLE auth_tokens DROP COLUMN from_person_id, DROP COLUMN sent_by_user_id;
COMMIT;
