-- Contact values leave the database only for the owner, admins, and rows marked 'family' (§5.2).

-- Serializes contact writes for one person, so the per-person cap holds; NO KEY, so a login link doesn't wait.
-- name: LockPersonForContacts :one
SELECT EXISTS (SELECT 1 FROM persons WHERE id = @id AND deleted_at IS NULL FOR NO KEY UPDATE);

-- name: PersonHasLogin :one
SELECT EXISTS (SELECT 1 FROM users WHERE person_id = @id);

-- name: CountContacts :one
SELECT count(*) FROM contact_methods WHERE person_id = @person_id;

-- name: ListContacts :many
SELECT id, kind, platform, label, value, visibility FROM contact_methods WHERE person_id = @person_id ORDER BY id;

-- name: ListFamilyContacts :many
SELECT id, kind, platform, label, value, visibility FROM contact_methods
WHERE person_id = @person_id AND visibility = 'family' ORDER BY id;

-- Only the kinds, never the values; someone without a login can't answer a request, so they offer none.
-- name: RequestableKinds :many
SELECT DISTINCT CASE WHEN kind = 'social' THEN platform ELSE kind END::text AS kind FROM contact_methods c
WHERE c.person_id = @person_id AND c.visibility = 'on_request'
  AND EXISTS (SELECT 1 FROM users u WHERE u.person_id = c.person_id)
ORDER BY 1;

-- name: GetContact :one
SELECT id, kind, platform, label, value, visibility FROM contact_methods WHERE id = @id AND person_id = @person_id;

-- name: CreateContact :one
INSERT INTO contact_methods (person_id, kind, platform, label, value, visibility)
VALUES (@person_id, @kind, sqlc.narg(platform), sqlc.narg(label), @value, @visibility)
RETURNING id, kind, platform, label, value, visibility;

-- name: UpdateContact :one
UPDATE contact_methods SET label = sqlc.narg(label), value = @value, visibility = @visibility, updated_at = now()
WHERE id = @id AND person_id = @person_id
RETURNING id, kind, platform, label, value, visibility;

-- name: DeleteContact :execrows
DELETE FROM contact_methods WHERE id = @id AND person_id = @person_id;
