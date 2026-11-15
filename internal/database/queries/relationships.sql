-- Every write takes this lock first. Relationship writes are rare, so serializing them is free, and the checks
-- below can't race: two links added at once can't close a loop together (§10).
-- name: LockRelationshipWrites :exec
SELECT pg_advisory_xact_lock(hashtext('family.relationships'));

-- name: CountPersonsInTree :one
SELECT count(*) FROM persons WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL;

-- name: RelationshipExists :one
SELECT EXISTS (SELECT 1 FROM relationships WHERE type = @type AND person_a = @person_a AND person_b = @person_b);

-- Every parent link counts, foster included, and so do deleted people, so no loop of any kind can be stored.
-- UNION drops people already visited, so the walk ends even on damaged data. The casts are for sqlc, which can't
-- infer a parameter's type inside a recursive CTE.
-- name: IsAncestor :one
WITH RECURSIVE anc(id) AS (
  SELECT person_a FROM relationships
  WHERE type = 'parent_child' AND person_b = @parent_id::uuid
  UNION
  SELECT r.person_a FROM relationships r
  JOIN anc ON r.person_b = anc.id
  WHERE r.type = 'parent_child'
)
SELECT EXISTS (SELECT 1 FROM anc WHERE id = @child_id::uuid);

-- Deleted people's links count too: restoring them must not make a third.
-- name: CountBiologicalParents :one
SELECT count(*) FROM relationships
WHERE type = 'parent_child' AND subtype = 'biological' AND person_b = @child_id AND id <> @except_id;

-- name: CreateRelationship :one
INSERT INTO relationships (type, subtype, ended, person_a, person_b, notes)
VALUES (@type, @subtype, @ended, @person_a, @person_b, sqlc.narg(notes))
RETURNING id, type, subtype, ended, person_a, person_b, notes;

-- name: LockRelationship :one
SELECT id, type, subtype, ended, person_a, person_b, notes FROM relationships WHERE id = @id FOR UPDATE;

-- name: UpdateRelationship :one
UPDATE relationships SET subtype = @subtype, ended = @ended, notes = sqlc.narg(notes), updated_at = now()
WHERE id = @id
RETURNING id, type, subtype, ended, person_a, person_b, notes;

-- name: DeleteRelationship :execrows
DELETE FROM relationships WHERE id = @id;
