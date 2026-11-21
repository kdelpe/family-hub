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
SELECT person_id, id, kind, platform, label, value, visibility FROM contact_methods
WHERE person_id = ANY(@person_ids::uuid[]) ORDER BY person_id, id;

-- What someone other than the owner sees: family rows, and the on_request rows of each kind the owner shared
-- with them (§5.2). A share counts only while both sides could still see it on their My profile: the owner
-- has an enabled login, and the requester's login is linked to someone in the tree, so it can be revoked.
-- name: ListVisibleContacts :many
SELECT c.person_id, c.id, c.kind, c.platform, c.label, c.value, c.visibility FROM contact_methods c
WHERE c.person_id = ANY(@person_ids::uuid[]) AND (c.visibility = 'family' OR (c.visibility = 'on_request'
  AND EXISTS (SELECT 1 FROM users u WHERE u.person_id = c.person_id AND u.disabled_at IS NULL)
  AND EXISTS (SELECT 1 FROM contact_requests r
    JOIN users ru ON ru.id = r.requester_id
    JOIN persons rp ON rp.id = ru.person_id AND rp.deleted_at IS NULL
    WHERE r.person_id = c.person_id AND r.requester_id = @requester_id AND r.status = 'shared'
      AND r.kind = CASE WHEN c.kind = 'social' THEN c.platform ELSE c.kind END)))
ORDER BY c.person_id, c.id;

-- The kinds the viewer could ask for and hasn't been shown, never their values. Declined and revoked read
-- as requested, so a decline stays silent; so does a share ListVisibleContacts no longer honours.
-- name: RequestableKinds :many
SELECT k.person_id, k.kind, CASE WHEN r.status IS NULL THEN 'none' ELSE 'requested' END::text AS my_request_status
FROM (SELECT DISTINCT c.person_id, CASE WHEN c.kind = 'social' THEN c.platform ELSE c.kind END::text AS kind FROM contact_methods c
      WHERE c.person_id = ANY(@person_ids::uuid[]) AND c.visibility = 'on_request'
        AND EXISTS (SELECT 1 FROM users u WHERE u.person_id = c.person_id AND u.disabled_at IS NULL)) k
LEFT JOIN contact_requests r ON r.person_id = k.person_id AND r.requester_id = @requester_id AND r.kind = k.kind
WHERE r.status IS DISTINCT FROM 'shared' OR NOT EXISTS (SELECT 1 FROM users ru
  JOIN persons rp ON rp.id = ru.person_id AND rp.deleted_at IS NULL WHERE ru.id = @requester_id)
ORDER BY k.person_id, k.kind;

-- The directory: every living person in the tree, for the Members book (design-notes §4.3), best matches first when searched.
-- Search matches names only, never contact values (§5.2). The limit is a backstop well above any real family.
-- name: DirectoryPersons :many
SELECT p.id, p.first_name, p.last_name, p.side,
  EXISTS (SELECT 1 FROM users u WHERE u.person_id = p.id AND u.disabled_at IS NULL) AS has_account
FROM persons p
WHERE p.deleted_at IS NULL AND NOT p.is_deceased
  AND (@q::text = '' OR p.search_name ILIKE '%' || @q_like::text || '%' ESCAPE '\' OR p.search_name % @q::text)
ORDER BY CASE WHEN @q::text = '' THEN 0 ELSE similarity(p.search_name, @q::text) END DESC, lower(p.search_name), p.id
LIMIT 20000;

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
