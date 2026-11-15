-- Every read selects explicit columns and the masked birth date, never birth_date itself (§10).

-- name: ListPersons :many
SELECT p.id, p.first_name, p.middle_name, p.last_name, p.maiden_name, p.side, p.birth_place, p.gender,
  -- sqlc types a function result as never NULL, so a hidden or missing date comes back as ''.
  coalesce(masked_birth_date(p, @viewer_is_admin::boolean, sqlc.narg(viewer_person_id)::uuid), '')::text AS birth_date,
  p.birth_date_precision, p.death_date, p.death_date_precision, p.is_deceased, p.bio
FROM persons p
WHERE p.deleted_at IS NULL
  AND (@q::text = '' OR p.search_name ILIKE '%' || @q_like::text || '%' ESCAPE '\' OR p.search_name % @q::text)
  AND (@side::text = '' OR p.side = @side::text)
  AND (sqlc.narg(deceased)::boolean IS NULL OR p.is_deceased = sqlc.narg(deceased)::boolean)
ORDER BY CASE WHEN @q::text = '' THEN 0 ELSE similarity(p.search_name, @q::text) END DESC, lower(p.search_name), p.id
LIMIT @max_rows;

-- name: GetPerson :one
SELECT p.id, p.first_name, p.middle_name, p.last_name, p.maiden_name, p.side, p.birth_place, p.gender,
  coalesce(masked_birth_date(p, @viewer_is_admin::boolean, sqlc.narg(viewer_person_id)::uuid), '')::text AS birth_date,
  p.birth_date_precision, p.death_date, p.death_date_precision, p.is_deceased, p.bio
FROM persons p
WHERE p.id = @id AND p.deleted_at IS NULL;

-- Admins only: the unmasked row, to apply a PATCH to.
-- name: LockPerson :one
SELECT id, first_name, middle_name, last_name, maiden_name, side, birth_place, gender,
  birth_date, birth_date_precision, death_date, death_date_precision, is_deceased, bio
FROM persons WHERE id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: CreatePerson :one
INSERT INTO persons (first_name, middle_name, last_name, maiden_name, side, birth_place, gender,
  birth_date, birth_date_precision, death_date, death_date_precision, is_deceased, bio, created_by)
VALUES (@first_name, sqlc.narg(middle_name), sqlc.narg(last_name), sqlc.narg(maiden_name), sqlc.narg(side),
  sqlc.narg(birth_place), sqlc.narg(gender), sqlc.narg(birth_date), sqlc.narg(birth_date_precision),
  sqlc.narg(death_date), sqlc.narg(death_date_precision), @is_deceased, sqlc.narg(bio), sqlc.narg(created_by))
RETURNING id;

-- name: UpdatePerson :exec
UPDATE persons SET first_name = @first_name, middle_name = sqlc.narg(middle_name), last_name = sqlc.narg(last_name),
  maiden_name = sqlc.narg(maiden_name), side = sqlc.narg(side), birth_place = sqlc.narg(birth_place),
  gender = sqlc.narg(gender), birth_date = sqlc.narg(birth_date), birth_date_precision = sqlc.narg(birth_date_precision),
  death_date = sqlc.narg(death_date), death_date_precision = sqlc.narg(death_date_precision),
  is_deceased = @is_deceased, bio = sqlc.narg(bio), updated_at = now()
WHERE id = @id;

-- name: SoftDeletePerson :execrows
UPDATE persons SET deleted_at = now(), updated_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: RestorePerson :execrows
UPDATE persons SET deleted_at = NULL, updated_at = now() WHERE id = @id AND deleted_at IS NOT NULL;
