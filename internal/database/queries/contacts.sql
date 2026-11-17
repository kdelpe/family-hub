-- Contact values leave the database only for the owner, admins, and rows marked 'family' (§5.2).

-- Serializes contact writes for one person, so the per-person cap holds; NO KEY, so a login link doesn't wait.
-- name: LockPersonForContacts :one
SELECT EXISTS (SELECT 1 FROM persons WHERE id = @id AND deleted_at IS NULL FOR NO KEY UPDATE);

-- Someone who could answer a request. The login is held until the commit, so a disable or unlink can't
-- land between this check and the write.
-- name: PersonHasLogin :one
SELECT EXISTS (SELECT 1 FROM users WHERE person_id = @id AND disabled_at IS NULL FOR SHARE);

-- name: CountContacts :one
SELECT count(*) FROM contact_methods WHERE person_id = @person_id;

-- name: ListContacts :many
SELECT id, kind, platform, label, value, visibility FROM contact_methods WHERE person_id = @person_id ORDER BY id;

-- What someone other than the owner sees: family rows, and the on_request rows of each kind the owner shared
-- with them (§5.2). A share counts only while both sides could still see it on their My profile: the owner
-- has an enabled login, and the requester's login is linked to someone in the tree, so it can be revoked.
-- name: ListVisibleContacts :many
SELECT c.id, c.kind, c.platform, c.label, c.value, c.visibility FROM contact_methods c
WHERE c.person_id = @person_id AND (c.visibility = 'family' OR (c.visibility = 'on_request'
  AND EXISTS (SELECT 1 FROM users u WHERE u.person_id = c.person_id AND u.disabled_at IS NULL)
  AND EXISTS (SELECT 1 FROM contact_requests r
    JOIN users ru ON ru.id = r.requester_id
    JOIN persons rp ON rp.id = ru.person_id AND rp.deleted_at IS NULL
    WHERE r.person_id = c.person_id AND r.requester_id = @requester_id AND r.status = 'shared'
      AND r.kind = CASE WHEN c.kind = 'social' THEN c.platform ELSE c.kind END)))
ORDER BY c.id;

-- The kinds the viewer could ask for and hasn't been shown, never their values. Declined and revoked read
-- as requested, so a decline stays silent.
-- name: RequestableKinds :many
SELECT k.kind, CASE WHEN r.status IS NULL THEN 'none' ELSE 'requested' END::text AS my_request_status
FROM (SELECT DISTINCT CASE WHEN c.kind = 'social' THEN c.platform ELSE c.kind END::text AS kind FROM contact_methods c
      WHERE c.person_id = @person_id AND c.visibility = 'on_request'
        AND EXISTS (SELECT 1 FROM users u WHERE u.person_id = c.person_id AND u.disabled_at IS NULL)) k
LEFT JOIN contact_requests r ON r.person_id = @person_id AND r.requester_id = @requester_id AND r.kind = k.kind
WHERE r.status IS DISTINCT FROM 'shared'
ORDER BY k.kind;

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
