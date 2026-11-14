-- name: CreateSession :exec
INSERT INTO sessions (id_hash, user_id, expires_at, user_agent)
VALUES (@id_hash, @user_id, now() + @lifetime::interval, @user_agent);

-- name: GetActiveSession :one
SELECT s.user_id, u.role, s.last_seen_at < now() - @touch_after::interval AS stale
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id_hash = @id_hash
  AND s.expires_at > now()
  AND s.last_seen_at > now() - @idle_timeout::interval
  AND u.disabled_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now()
WHERE id_hash = @id_hash AND last_seen_at < now() - @touch_after::interval;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id_hash = @id_hash;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: SessionActive :one
SELECT EXISTS (SELECT 1 FROM sessions WHERE id_hash = @id_hash AND user_id = @user_id
  AND expires_at > clock_timestamp() AND last_seen_at > clock_timestamp() - @idle_timeout::interval);

-- name: LockActiveSession :one
-- Waits out a logout deleting the row, which takes no user lock.
SELECT EXISTS (SELECT 1 FROM sessions WHERE id_hash = @id_hash AND user_id = @user_id
  AND expires_at > clock_timestamp() AND last_seen_at > clock_timestamp() - @idle_timeout::interval FOR UPDATE);
