-- While a request for the email is pending, a new one is dropped and the pending one left as it was (§5).
-- name: CreateAccessRequest :exec
INSERT INTO access_requests (full_name, email, side, relationship, related_names, note, source, suggested_by)
VALUES (@full_name, @email, sqlc.narg(side), sqlc.narg(relationship), @related_names::text[], sqlc.narg(note), @source, sqlc.narg(suggested_by))
ON CONFLICT (email) WHERE status = 'pending' DO NOTHING;

-- The Requests queue: pending first, then the ones declined in the last 90 days, which can still be reopened.
-- name: ListAccessRequests :many
SELECT r.id, r.full_name, r.email, r.side, r.relationship, r.related_names, r.note, r.source, r.status,
  r.reviewed_at, r.created_at, sp.id AS suggester_person_id, sp.first_name AS suggester_first_name, sp.last_name AS suggester_last_name
FROM access_requests r
LEFT JOIN users su ON su.id = r.suggested_by
LEFT JOIN persons sp ON sp.id = su.person_id AND sp.deleted_at IS NULL
WHERE r.status = 'pending' OR (r.status = 'declined' AND r.reviewed_at > now() - interval '90 days')
ORDER BY r.status = 'declined', r.created_at, r.id;

-- The closest person in the tree to each name that has one close enough (pg_trgm's % threshold), by the
-- name's position in the list. Only admins see these, never the public form (§12).
-- name: MatchRelatedNames :many
SELECT (n.i - 1)::int AS idx, m.id AS person_id, m.first_name, m.last_name
FROM unnest(@names::text[]) WITH ORDINALITY AS n(name, i)
JOIN LATERAL (
  SELECT p.id, p.first_name, p.last_name FROM persons p
  WHERE p.deleted_at IS NULL AND p.search_name % n.name
  ORDER BY similarity(p.search_name, n.name) DESC, p.id
  LIMIT 1
) m ON true;

-- name: DeclineAccessRequest :execrows
UPDATE access_requests SET status = 'declined', reviewed_by = @reviewed_by, reviewed_at = now()
WHERE id = @id AND status = 'pending';

-- name: ReopenAccessRequest :execrows
UPDATE access_requests SET status = 'pending', reviewed_by = NULL, reviewed_at = NULL
WHERE id = @id AND status = 'declined';

-- name: AccessRequestExists :one
SELECT EXISTS (SELECT 1 FROM access_requests WHERE id = @id);

-- Held until the invite commits, so two admins can't both approve it.
-- name: LockPendingAccessRequest :one
SELECT EXISTS (SELECT 1 FROM access_requests WHERE id = @id AND status = 'pending' FOR UPDATE);

-- name: ApproveAccessRequest :exec
UPDATE access_requests SET status = 'approved', reviewed_by = @reviewed_by, reviewed_at = now(), invited_user_id = @invited_user_id
WHERE id = @id;

-- Declined requests are kept 90 days for Undo, then deleted by the cleanup job (§5).
-- name: DeleteOldDeclinedAccessRequests :execrows
DELETE FROM access_requests WHERE id IN (
  SELECT id FROM access_requests WHERE status = 'declined' AND reviewed_at < now() - interval '90 days'
  ORDER BY reviewed_at LIMIT @batch FOR UPDATE SKIP LOCKED);
