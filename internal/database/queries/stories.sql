-- The teller is a name only, as for an event's people (§10); a teller deleted from the tree drops out.
-- name: GetStory :one
SELECT s.id, s.title, s.status, s.consent_confirmed_at, s.recorded_on, s.recorded_on_precision, s.place, s.summary,
  s.language, s.chapters, s.transcript, s.topics, s.other_people, s.featured_at, s.created_by, s.created_at,
  p.id AS teller_id, p.first_name AS teller_first_name, p.last_name AS teller_last_name
FROM stories s
LEFT JOIN persons p ON p.id = s.teller_person_id AND p.deleted_at IS NULL
WHERE s.id = @id AND s.deleted_at IS NULL;

-- Published: the story of the week first, then the latest added. Drafts: the given creator's, or everyone's.
-- Chapters and transcripts are left out: a list shows neither, and a transcript can be long.
-- name: ListStories :many
SELECT s.id, s.title, s.status, s.consent_confirmed_at, s.recorded_on, s.recorded_on_precision, s.place, s.summary,
  s.language, '[]'::jsonb AS chapters, '[]'::jsonb AS transcript, s.topics, s.other_people, s.featured_at, s.created_by, s.created_at,
  p.id AS teller_id, p.first_name AS teller_first_name, p.last_name AS teller_last_name
FROM stories s
LEFT JOIN persons p ON p.id = s.teller_person_id AND p.deleted_at IS NULL
WHERE s.deleted_at IS NULL AND s.status = @status
  AND (sqlc.narg(created_by)::uuid IS NULL OR s.created_by = sqlc.narg(created_by))
  AND (sqlc.narg(teller_id)::uuid IS NULL OR p.id = sqlc.narg(teller_id))
  AND (sqlc.narg(topic)::text IS NULL OR lower(sqlc.narg(topic)) = ANY(SELECT lower(t) FROM unnest(s.topics) t))
  -- A search of punctuation alone parses to an empty query, which would match nothing: it searches for nothing instead.
  AND (sqlc.narg(q)::text IS NULL OR numnode(websearch_to_tsquery('simple', sqlc.narg(q))) = 0
    OR s.search_tsv @@ websearch_to_tsquery('simple', sqlc.narg(q)))
ORDER BY s.featured_at DESC NULLS LAST, s.created_at DESC, s.id DESC;

-- name: ListStoryPeople :many
SELECT sp.story_id, p.id, p.first_name, p.last_name
FROM story_persons sp
JOIN persons p ON p.id = sp.person_id AND p.deleted_at IS NULL
WHERE sp.story_id = ANY(@story_ids::uuid[])
ORDER BY lower(p.search_name), p.id;

-- name: CreateStory :one
INSERT INTO stories (title, teller_person_id, consent_confirmed_at, recorded_on, recorded_on_precision, place, summary,
  language, chapters, transcript, topics, other_people, created_by)
VALUES (@title, sqlc.narg(teller_person_id), sqlc.narg(consent_confirmed_at), sqlc.narg(recorded_on),
  sqlc.narg(recorded_on_precision), sqlc.narg(place), sqlc.narg(summary), sqlc.narg(language), @chapters, @transcript,
  @topics, @other_people, sqlc.narg(created_by))
RETURNING id;

-- name: LockStory :one
SELECT id, title, teller_person_id, status, consent_confirmed_at, recorded_on, recorded_on_precision, place, summary,
  language, chapters, transcript, topics, other_people, featured_at, created_by
FROM stories WHERE id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: UpdateStory :exec
UPDATE stories SET title = @title, teller_person_id = sqlc.narg(teller_person_id), status = @status,
  consent_confirmed_at = sqlc.narg(consent_confirmed_at), recorded_on = sqlc.narg(recorded_on),
  recorded_on_precision = sqlc.narg(recorded_on_precision), place = sqlc.narg(place), summary = sqlc.narg(summary),
  language = sqlc.narg(language), chapters = @chapters, transcript = @transcript, topics = @topics,
  other_people = @other_people, featured_at = sqlc.narg(featured_at), updated_at = now()
WHERE id = @id;

-- Featuring takes this lock first, so the next one waits and then sees this one's story to unfeature.
-- name: LockStoryOfTheWeek :exec
SELECT pg_advisory_xact_lock(hashtext('story of the week'));

-- name: UnfeatureOtherStories :exec
UPDATE stories SET featured_at = NULL, updated_at = now() WHERE featured_at IS NOT NULL AND id <> @id;

-- name: SoftDeleteStory :exec
UPDATE stories SET deleted_at = now(), featured_at = NULL, updated_at = now() WHERE id = @id;

-- name: ClearStoryPeople :exec
DELETE FROM story_persons WHERE story_id = @story_id;

-- name: AddStoryPeople :exec
INSERT INTO story_persons (story_id, person_id) SELECT @story_id, unnest(@person_ids::uuid[]);
