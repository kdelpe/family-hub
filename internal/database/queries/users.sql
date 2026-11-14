-- name: GetLoginUser :one
SELECT id, password_hash, password_changed_at, disabled_at FROM users WHERE email = @email;

-- name: LockUser :one
SELECT email, role, password_changed_at, disabled_at, google_sub FROM users WHERE id = @id FOR UPDATE;

-- name: GetGoogleUserBySub :one
SELECT id FROM users WHERE google_sub = @google_sub;

-- name: GetInvitedGoogleUser :one
SELECT id FROM users WHERE email = @email AND disabled_at IS NULL
	AND password_hash IS NULL AND google_sub IS NULL;

-- name: LinkGoogleAndRecordLogin :exec
UPDATE users SET google_sub = @google_sub, last_login_at = clock_timestamp() WHERE id = @id;

-- name: RecordGoogleLogin :exec
UPDATE users SET last_login_at = clock_timestamp() WHERE id = @id;

-- name: RecordLogin :exec
UPDATE users
SET last_login_at = now(), password_hash = coalesce(sqlc.narg(new_password_hash), password_hash)
WHERE id = @id;

-- name: GetUser :one
SELECT id, email, role FROM users WHERE id = @id;

-- name: CreateAdmin :exec
INSERT INTO users (email, role, password_hash, password_changed_at)
VALUES (@email, 'admin', @password_hash::text, now());

-- name: EmailTaken :one
SELECT EXISTS (SELECT 1 FROM users WHERE email = @email);

-- name: SetPasswordAndRecordLogin :exec
UPDATE users SET password_hash = @password_hash::text, password_changed_at = clock_timestamp(), last_login_at = clock_timestamp() WHERE id = @id;

-- name: CreateInvitedUser :one
INSERT INTO users (email, role) VALUES (@email, @role) RETURNING id;

-- name: LockEnabledUserByEmail :one
SELECT id FROM users WHERE email = @email AND disabled_at IS NULL FOR UPDATE;

-- name: GetUserPassword :one
SELECT email, password_hash, password_changed_at, disabled_at FROM users WHERE id = @id;

-- name: SetPassword :exec
UPDATE users SET password_hash = @password_hash::text, password_changed_at = clock_timestamp() WHERE id = @id;
