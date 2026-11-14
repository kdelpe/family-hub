-- name: CreateAuthToken :exec
INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at)
VALUES (@user_id, @purpose, @token_hash, now() + @ttl::interval);

-- name: GetUsableToken :one
SELECT t.id, t.user_id, u.email
FROM auth_tokens t JOIN users u ON u.id = t.user_id
WHERE t.token_hash = @token_hash AND t.consumed_at IS NULL AND t.expires_at > now() AND u.disabled_at IS NULL;

-- name: ConsumeToken :execrows
UPDATE auth_tokens SET consumed_at = now()
WHERE id = @id AND consumed_at IS NULL AND expires_at > now();

-- name: DeleteOtherTokens :exec
DELETE FROM auth_tokens WHERE user_id = @user_id AND id <> @keep_id;
