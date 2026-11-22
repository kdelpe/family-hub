-- No dates and no contacts: the tree never shows them, so they can't leak through it (§10).
-- name: TreePersons :many
SELECT p.id, p.first_name, p.last_name, p.side, p.gender, p.is_deceased,
  -- sqlc types a function result as never NULL, so no bio comes back as ''.
  coalesce(CASE WHEN length(p.bio) > 140 THEN rtrim(left(p.bio, 139), E' \t\r\n') || '…' ELSE p.bio END, '')::text AS bio_excerpt
FROM persons p
WHERE p.deleted_at IS NULL
ORDER BY lower(p.search_name), p.id;

-- No notes: they're the admins' working notes, not part of the tree.
-- name: TreeRelationships :many
SELECT r.id, r.type, r.subtype, r.ended, r.person_a, r.person_b
FROM relationships r
JOIN persons a ON a.id = r.person_a AND a.deleted_at IS NULL
JOIN persons b ON b.id = r.person_b AND b.deleted_at IS NULL
ORDER BY r.id;

-- Family-parent links only, so a foster parent is never an ancestor (§3.4); the walks stop at deleted people.
-- UNION keeps one row per person and depth. The casts are for sqlc, which can't type parameters in a recursive CTE.
-- name: Ancestors :many
WITH RECURSIVE anc(id, depth) AS (
  SELECT r.person_a, 1
  FROM relationships r
  JOIN persons p ON p.id = r.person_a AND p.deleted_at IS NULL
  WHERE r.type = 'parent_child' AND r.subtype IN ('biological', 'adoptive') AND r.person_b = @person_id::uuid
  UNION
  SELECT r.person_a, anc.depth + 1
  FROM anc
  JOIN relationships r ON r.person_b = anc.id
  JOIN persons p ON p.id = r.person_a AND p.deleted_at IS NULL
  WHERE r.type = 'parent_child' AND r.subtype IN ('biological', 'adoptive') AND anc.depth < @max_depth::int
)
SELECT p.id, p.first_name, p.last_name, p.is_deceased,
  coalesce(masked_birth_date(p, @viewer_is_admin::boolean, sqlc.narg(viewer_person_id)::uuid), '')::text AS birth_date,
  p.birth_date_precision, a.depth
FROM (SELECT id, min(depth)::int AS depth FROM anc GROUP BY id) a
JOIN persons p ON p.id = a.id
ORDER BY a.depth, lower(p.search_name), p.id;

-- name: Descendants :many
WITH RECURSIVE des(id, depth) AS (
  SELECT r.person_b, 1
  FROM relationships r
  JOIN persons p ON p.id = r.person_b AND p.deleted_at IS NULL
  WHERE r.type = 'parent_child' AND r.subtype IN ('biological', 'adoptive') AND r.person_a = @person_id::uuid
  UNION
  SELECT r.person_b, des.depth + 1
  FROM des
  JOIN relationships r ON r.person_a = des.id
  JOIN persons p ON p.id = r.person_b AND p.deleted_at IS NULL
  WHERE r.type = 'parent_child' AND r.subtype IN ('biological', 'adoptive') AND des.depth < @max_depth::int
)
SELECT p.id, p.first_name, p.last_name, p.is_deceased,
  coalesce(masked_birth_date(p, @viewer_is_admin::boolean, sqlc.narg(viewer_person_id)::uuid), '')::text AS birth_date,
  p.birth_date_precision, d.depth
FROM (SELECT id, min(depth)::int AS depth FROM des GROUP BY id) d
JOIN persons p ON p.id = d.id
ORDER BY d.depth, lower(p.search_name), p.id;

-- name: CountPersons :one
SELECT count(*) FROM persons WHERE deleted_at IS NULL;

-- name: FamilyParentLinks :many
SELECT r.person_a AS parent_id, r.person_b AS child_id
FROM relationships r
JOIN persons a ON a.id = r.person_a AND a.deleted_at IS NULL
JOIN persons b ON b.id = r.person_b AND b.deleted_at IS NULL
WHERE r.type = 'parent_child' AND r.subtype IN ('biological', 'adoptive');

-- Only the deceased count, so no living person's birth year shows (§5.2).
-- name: EarliestRecord :one
SELECT birth_date, birth_date_precision, birth_place
FROM persons
WHERE deleted_at IS NULL AND is_deceased AND birth_date IS NOT NULL
ORDER BY birth_date, id
LIMIT 1;

-- name: BirthdaysOn :many
-- Living people whose birthday ("MM-DD") is one of the given days. Only the month and day leave the database,
-- never the year (§5.3).
SELECT id, first_name, last_name, to_char(birth_date, 'MM-DD')::text AS month_day
FROM persons
WHERE deleted_at IS NULL AND NOT is_deceased AND birth_date_precision = 'day'
  AND to_char(birth_date, 'MM-DD') = ANY(@month_days::text[]);

-- name: AddedSince :many
-- Not the viewer: their own arrival isn't news to them.
SELECT id, first_name, last_name, created_at
FROM persons
WHERE deleted_at IS NULL AND created_at >= @since AND id IS DISTINCT FROM sqlc.narg(viewer_person_id)::uuid
ORDER BY created_at DESC, id DESC
LIMIT @max_people;

-- name: FamilyCounts :one
-- Generations is the longest parent line, birth and adoptive links only as the tree summary counts them;
-- the depth cap stops a cycle, which the service never stores.
WITH RECURSIVE living AS (
  SELECT id, created_at AS added_at FROM persons WHERE deleted_at IS NULL
),
line AS (
  SELECT l.id, 1 AS depth
  FROM living l
  WHERE NOT EXISTS (
    SELECT 1 FROM relationships r JOIN living p ON p.id = r.person_a
    WHERE r.type = 'parent_child' AND r.subtype IN ('biological', 'adoptive') AND r.person_b = l.id
  )
  UNION
  SELECT r.person_b, line.depth + 1
  FROM line
  JOIN relationships r ON r.type = 'parent_child' AND r.subtype IN ('biological', 'adoptive') AND r.person_a = line.id
  JOIN living c ON c.id = r.person_b
  WHERE line.depth < 64
)
SELECT
  (SELECT count(*) FROM living)::int AS people,
  (SELECT coalesce(max(depth), 0) FROM line)::int AS generations,
  (SELECT count(*) FROM living WHERE added_at >= @since::timestamptz)::int AS added_since;
