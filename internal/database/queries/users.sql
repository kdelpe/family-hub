-- name: GetLoginUser :one
SELECT id, password_hash, password_changed_at, disabled_at FROM users WHERE email = @email;

-- name: LockUser :one
SELECT email, role, password_changed_at, disabled_at, (password_hash IS NOT NULL)::boolean AS set_up, person_id
FROM users WHERE id = @id FOR UPDATE;

-- name: RecordLogin :exec
UPDATE users
SET last_login_at = now(), password_hash = coalesce(sqlc.narg(new_password_hash), password_hash)
WHERE id = @id;

-- name: GetUser :one
SELECT id, email, role, person_id FROM users WHERE id = @id;

-- name: CreateAdmin :exec
INSERT INTO users (email, role, password_hash, password_changed_at)
VALUES (@email, 'admin', @password_hash::text, now());

-- name: EmailTaken :one
SELECT EXISTS (SELECT 1 FROM users WHERE email = @email);

-- name: SetPasswordAndRecordLogin :exec
UPDATE users SET password_hash = @password_hash::text, password_changed_at = clock_timestamp(), last_login_at = clock_timestamp() WHERE id = @id;

-- name: CreateInvitedUser :one
INSERT INTO users (email, role, person_id) VALUES (@email, @role, @person_id) RETURNING id;

-- name: LockEnabledUserByEmail :one
SELECT id FROM users WHERE email = @email AND disabled_at IS NULL FOR UPDATE;

-- name: GetUserPassword :one
SELECT email, password_hash, password_changed_at, disabled_at FROM users WHERE id = @id;

-- name: SetPassword :exec
UPDATE users SET password_hash = @password_hash::text, password_changed_at = clock_timestamp() WHERE id = @id;

-- name: ListUsers :many
SELECT u.id, u.email, u.role, u.disabled_at, u.last_login_at, u.created_at,
  (u.password_hash IS NOT NULL)::boolean AS set_up, u.person_id, p.first_name AS person_first_name, p.last_name AS person_last_name,
  -- sqlc can't type a nullable max(), so has_invite says whether invite_expires_at is real.
  (count(t.id) > 0)::boolean AS has_invite, coalesce(max(t.expires_at), u.created_at)::timestamptz AS invite_expires_at
FROM users u
LEFT JOIN auth_tokens t ON t.user_id = u.id AND t.purpose = 'invite' AND t.consumed_at IS NULL
LEFT JOIN persons p ON p.id = u.person_id
WHERE @after_email::text = '' OR (u.email, u.id) > (@after_email::text, @after_id::uuid)
GROUP BY u.id, p.id
ORDER BY u.email, u.id
LIMIT @page_size;

-- name: GetAccount :one
SELECT u.id, u.email, u.role, u.disabled_at, u.last_login_at, u.created_at,
  (u.password_hash IS NOT NULL)::boolean AS set_up, u.person_id, p.first_name AS person_first_name, p.last_name AS person_last_name,
  (count(t.id) > 0)::boolean AS has_invite, coalesce(max(t.expires_at), u.created_at)::timestamptz AS invite_expires_at
FROM users u
LEFT JOIN auth_tokens t ON t.user_id = u.id AND t.purpose = 'invite' AND t.consumed_at IS NULL
LEFT JOIN persons p ON p.id = u.person_id
WHERE u.id = @id
GROUP BY u.id, p.id;

-- Only admins who can sign in: an invited admin who never set up can't manage anyone.
-- name: CountUsableAdmins :one
SELECT count(*) FROM users
WHERE role = 'admin' AND disabled_at IS NULL AND password_hash IS NOT NULL;

-- name: SetRoleAndDisabled :exec
UPDATE users
SET role = @role, disabled_at = CASE WHEN @disabled::boolean THEN coalesce(disabled_at, clock_timestamp()) END
WHERE id = @id;

-- The old address's password mustn't carry over to whoever owns the new one (§5).
-- name: ChangeEmail :exec
UPDATE users SET email = @email, password_hash = NULL, password_changed_at = NULL WHERE id = @id;

-- name: SetUserPerson :exec
UPDATE users SET person_id = sqlc.narg(person_id) WHERE id = @id;

-- Held until the commit, so the person can't be deleted while a login is being linked to them. KEY SHARE
-- only blocks that delete, not edits to the person during the invite's email send.
-- name: LockPersonToLink :one
SELECT EXISTS (SELECT 1 FROM persons WHERE id = @id AND deleted_at IS NULL FOR KEY SHARE);
