-- Serializes one requester's asks, so the daily limit holds.
-- name: LockRequester :exec
SELECT pg_advisory_xact_lock(hashtext('contacts.requests'), hashtext(CAST(@requester_id::uuid AS text)));

-- Retry-after comes from the database's clock, the one created_at was stamped with.
-- name: RequestsToday :one
SELECT count(*) AS n,
  extract(epoch FROM coalesce(min(created_at), now()) + interval '1 day' - now())::float8 AS retry_after_seconds
FROM contact_requests
WHERE requester_id = @requester_id AND created_at > now() - interval '1 day';

-- name: GetMyRequestStatus :one
SELECT status FROM contact_requests WHERE requester_id = @requester_id AND person_id = @person_id AND kind = @kind;

-- name: CreateContactRequest :exec
INSERT INTO contact_requests (requester_id, person_id, kind) VALUES (@requester_id, @person_id, @kind);

-- Requests for one person's details, from people who can still sign in: pending first, then the shares.
-- name: ListRequestsFor :many
SELECT r.id, r.kind, r.status, r.created_at, r.decided_at, p.id AS requester_person_id, p.first_name, p.last_name
FROM contact_requests r
JOIN users u ON u.id = r.requester_id AND u.disabled_at IS NULL
JOIN persons p ON p.id = u.person_id AND p.deleted_at IS NULL
WHERE r.person_id = @person_id
ORDER BY r.status <> 'pending', r.created_at, r.id;

-- name: DecideContactRequest :execrows
UPDATE contact_requests SET status = @to_status, decided_at = now()
WHERE id = @id AND person_id = @person_id AND status = @from_status;

-- name: ContactRequestExists :one
SELECT EXISTS (SELECT 1 FROM contact_requests WHERE id = @id AND person_id = @person_id);

-- An on_request row of that kind; Ask checks the login to answer it separately, under a lock.
-- name: OffersKind :one
SELECT EXISTS (SELECT 1 FROM contact_methods c
  WHERE c.person_id = @person_id AND c.visibility = 'on_request' AND CASE WHEN c.kind = 'social' THEN c.platform ELSE c.kind END = @kind::text);
