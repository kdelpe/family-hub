-- Newest first; undated events go last, latest added first.
-- name: ListEvents :many
SELECT id, title, event_date, event_date_precision, location, description, created_at
FROM events
WHERE deleted_at IS NULL
ORDER BY event_date DESC NULLS LAST, created_at DESC, id DESC;

-- name: GetEvent :one
SELECT id, title, event_date, event_date_precision, location, description, created_at
FROM events
WHERE id = @id AND deleted_at IS NULL;

-- Names only, never dates or contacts (§10); people deleted from the tree drop out.
-- name: ListEventPeople :many
SELECT ep.event_id, p.id, p.first_name, p.last_name
FROM event_persons ep
JOIN persons p ON p.id = ep.person_id AND p.deleted_at IS NULL
WHERE ep.event_id = ANY(@event_ids::uuid[])
ORDER BY lower(p.search_name), p.id;

-- name: CreateEvent :one
INSERT INTO events (title, event_date, event_date_precision, location, description, created_by)
VALUES (@title, sqlc.narg(event_date), sqlc.narg(event_date_precision), sqlc.narg(location), sqlc.narg(description),
  sqlc.narg(created_by))
RETURNING id;

-- name: LockEvent :one
SELECT id, title, event_date, event_date_precision, location, description
FROM events WHERE id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: UpdateEvent :exec
UPDATE events SET title = @title, event_date = sqlc.narg(event_date), event_date_precision = sqlc.narg(event_date_precision),
  location = sqlc.narg(location), description = sqlc.narg(description), updated_at = now()
WHERE id = @id;

-- name: SoftDeleteEvent :execrows
UPDATE events SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ClearEventPeople :exec
DELETE FROM event_persons WHERE event_id = @event_id;

-- name: AddEventPeople :exec
INSERT INTO event_persons (event_id, person_id) SELECT @event_id, unnest(@person_ids::uuid[]);

-- The people to tag, locked until commit, in id order so two writes can't deadlock. FOR KEY SHARE waits for a
-- delete in progress (LockPersonToDelete's FOR UPDATE), then sees whether it happened.
-- name: LockPersonsToTag :many
SELECT id FROM persons WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL ORDER BY id FOR KEY SHARE;
