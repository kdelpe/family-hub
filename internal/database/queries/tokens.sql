-- name: GetUsableToken :one
SELECT t.id, t.user_id, t.purpose, u.email
FROM auth_tokens t JOIN users u ON u.id = t.user_id
WHERE t.token_hash = @token_hash AND t.consumed_at IS NULL AND t.expires_at > now() AND u.disabled_at IS NULL;

-- Deletes all the user's tokens and reports whether the presented one was still usable;
-- the caller rolls back if not. clock_timestamp, as now() is when the transaction began, before its wait for the lock.
-- name: UseToken :one
WITH deleted AS (DELETE FROM auth_tokens WHERE user_id = @user_id RETURNING id, consumed_at, expires_at)
SELECT EXISTS (SELECT 1 FROM deleted WHERE id = @id AND consumed_at IS NULL AND expires_at > clock_timestamp());

-- name: CreateToken :exec
INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
VALUES (@user_id, @purpose, @token_hash, now() + @lifetime::interval);

-- name: DeleteUserTokens :exec
DELETE FROM auth_tokens WHERE user_id = @user_id;

-- name: DeleteOldExpiredTokens :execrows
WITH doomed AS (
	SELECT id FROM auth_tokens WHERE expires_at < clock_timestamp() - interval '7 days'
	ORDER BY expires_at LIMIT @batch_size FOR UPDATE SKIP LOCKED
)
DELETE FROM auth_tokens t USING doomed WHERE t.id = doomed.id;
