-- Who an invite is from (in the email and on the Welcome page) and who sent it (§5). NULL for password resets.
ALTER TABLE auth_tokens
  ADD COLUMN from_person_id  UUID REFERENCES persons(id) ON DELETE SET NULL,
  ADD COLUMN sent_by_user_id UUID REFERENCES users(id)   ON DELETE SET NULL;
