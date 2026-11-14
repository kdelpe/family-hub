-- name: GetUsableToken :one
SELECT t.id, t.user_id, u.email
FROM auth_tokens t JOIN users u ON u.id = t.user_id
WHERE t.token_hash = @token_hash AND t.expires_at > now() AND u.disabled_at IS NULL;

-- Deletes all the user's tokens and reports whether the presented one was still there and unexpired;
-- the caller rolls back if not. clock_timestamp, as now() is when the transaction began, before its wait for the lock.
-- name: UseToken :one
WITH deleted AS (DELETE FROM auth_tokens WHERE user_id = @user_id RETURNING id, expires_at)
SELECT EXISTS (SELECT 1 FROM deleted WHERE id = @id AND expires_at > clock_timestamp());
