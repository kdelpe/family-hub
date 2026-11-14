-- name: GetUsableToken :one
SELECT t.id, t.user_id, u.email
FROM auth_tokens t JOIN users u ON u.id = t.user_id
WHERE t.token_hash = @token_hash AND t.consumed_at IS NULL AND t.expires_at > now() AND u.disabled_at IS NULL;

-- clock_timestamp, not now(): now() is when the transaction began, before its wait for the user lock.
-- name: ConsumeToken :execrows
UPDATE auth_tokens SET consumed_at = now()
WHERE id = @id AND consumed_at IS NULL AND expires_at > clock_timestamp();

-- name: DeleteUserTokens :exec
DELETE FROM auth_tokens WHERE user_id = @user_id;
