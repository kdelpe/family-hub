# Family Hub: Architecture & Engineering Plan V3

A private, invite-only, multi-generational family archive: interactive family tree, event/milestone feed, photo and video library, recorded family stories, sign-in with email + password or Google. Relatives can ask for an invitation; an admin decides. The frontend's look and behaviour are specified in `docs/design/` (§7). One Go service serves the JSON API and the embedded React SPA. Postgres holds data; an S3-compatible bucket holds media, uploaded directly from the browser.

**Status:** draft for review. Items marked **(verify)** are things I believe but haven't confirmed against current docs; check each one when the phase that depends on it starts.

---

## 0. What changed from V2

| # | V2 problem | V3 fix |
|---|---|---|
| 1 | `ui.go` was `package main` at repo root; `cmd/server` can't import it and `go:embed` can't reach `..` | Embed lives in `internal/webui` (package `webui`); Vite builds into it (§2, §7) |
| 2 | Fresh clone fails `go build`/`go test` because `dist/` doesn't exist | `make ui-stub` writes a placeholder; `dist/` is git-ignored (§8.1) |
| 3 | Makefile `POSTGRES_URL` garbled in the PDF | Rewritten; migrations also run via the binary itself (§8) |
| 4 | 6-digit code: brute-forceable, replayable, bare SHA-256, no session storage | Email + password (Argon2id) or Google sign-in; 256-bit single-use invite/reset tokens; server-side sessions, rate limits, enumeration-safe (§5) |
| 5 | `RealIP` trusted unconditionally | Behind a configured trusted proxy only, and from the proxy's own `X-Forwarded-For` entry (§6) |
| 6 | No way to *view* media; no write endpoints | Full endpoint table incl. presigned-GET and CRUD (§4) |
| 7 | `confirm` trusted the client | `HeadObject` verify; Content-Length and Content-Type signed into the PUT; orphan cleanup (§9) |
| 8 | HEVC `.mov` won't play in most browsers | Processing worker (ffmpeg) with poster + H.264 variant, phased in (§9.4, §15) |
| 9 | Relationships: no uniqueness, ambiguous direction, wrong index | Directional convention, `UNIQUE`, canonical ordering, both-side indexes (§3) |
| 10 | `DATE` can't express "circa 1950" | Date + precision columns (§3) |
| 11 | Events not linked to people | `event_persons` (§3) |
| 12 | Unnecessary `uuid-ossp`; two Postgres drivers (`lib/pq` via migrate + pgx) | Dropped; migrate uses its pgx v5 driver (§6) |
| 13 | Recursive CTE for the whole tree | `/api/tree` is one flat read; CTEs only for ancestor/descendant queries (§10) |
| 14 | No backups, no email provider, no CI, root container | §11, §6, §13, §8 |
| 15 | MinIO assumed for local dev | Storage server is an explicit decision at the start of Phase 5 (§9.5) |

---

## 1. Principles

1. **One deployable.** A single container image runs the API and serves the SPA. (V2 called it one "immutable binary"; ffmpeg in Phase 6 makes it image-level, not binary-level. That's fine.)
2. **Boring tools.** Go + chi + pgx + sqlc, Postgres, golang-migrate, React + Vite, S3 API. No framework magic, no ORM.
3. **Bytes never touch the app.** Uploads and downloads go browser ↔ bucket via presigned URLs.
4. **Family data is precious.** Soft deletes, tested backups, and no synchronous object deletion.
5. **Local-first, cloud-later.** Every external dependency (DB, bucket, mail) is configured by env vars only.

## 2. Stack & repository layout

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.27 (`go 1.27` in `go.mod`) | 1.25 is the floor for `CrossOriginProtection` and `synctest`; `go tool` directives need ≥1.24. |
| HTTP | chi v5 | stdlib `net/http` compatible |
| Logs | zerolog | JSON on stderr; each request's logger travels in its context (`zerolog.Ctx`) |
| DB access | pgx v5 pool + **sqlc** | Typed queries from `.sql` files; no runtime reflection. Generated types are plain Go: `uuid.UUID` (`github.com/google/uuid`), `time.Time`, `time.Duration` for intervals, and pointers for nullable columns (`sqlc.yaml` overrides) |
| Migrations | golang-migrate, embedded, pgx v5 driver | Auto-run gated by `AUTO_MIGRATE` |
| Storage | S3 API via aws-sdk-go-v2 | Local server chosen at the start of Phase 5 (§9.5); R2 in prod |
| Mail | SMTP (`net/smtp`) behind a `Mailer` interface | Mailpit local; Resend/SES/Postmark SMTP later |
| Frontend | React 19 + TypeScript + Vite, TanStack Query | Focus-view family tree (§7); plain CSS, look and pages from `docs/design/` |
| Jobs | Postgres queue (`FOR UPDATE SKIP LOCKED`) in-process | No Redis/SQS needed at this scale; added in Phase 6 |
| Auth | Argon2id via `golang.org/x/crypto/argon2`, NFC normalization via `golang.org/x/text`; Google sign-in via `golang.org/x/oauth2` + `github.com/coreos/go-oidc/v3`; hidden password prompt for `admin create` via `golang.org/x/term` | The `x/` packages are maintained by the Go team |
| Stdlib where nothing else is needed | `flag` (subcommands), `os.Getenv` + validation (config), `http.CrossOriginProtection` (CSRF, Go 1.25), `crypto/*` | No cobra, no config library |
| Testing | `testing` + **testify** (assertions, mocks) + **mockery** (generated mocks) + `testing/synctest` (fake clock, Go 1.25) | Database is never mocked: real Postgres in integration tests (§13) |
| Lint & security | golangci-lint, `govulncheck` | |
| CI / releases | GitHub Actions | `.github/` (§13.1) |

```
family-hub/
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                # checks on PRs into main and pushes to main (§13.1)
│   │   ├── draft-release.yml     # keeps one draft Release with generated notes, updated on each push to main (§13.1)
│   │   └── release.yml           # builds the image when a Release is published from main (§13.1)
│   ├── pull_request_template.md
│   ├── CODEOWNERS
│   └── dependabot.yml            # Go, npm, Actions, Docker update PRs -> main
├── cmd/main.go                   # subcommands: serve (default) | migrate up | admin create; built with -o family-hub
├── internal/
│   ├── app/                      # builds dependencies (pool, services, handlers), server lifecycle, graceful shutdown
│   ├── router/                   # chi route table + middleware chain; the only place routes are wired (§6.1)
│   ├── handler/                  # all HTTP handlers, one file per area: health.go auth.go admin.go
│   │                             #   family.go contacts.go event.go media.go; service interfaces they use
│   │   └── mocks/                # mockery mocks of those service interfaces
│   ├── config/                   # env loader + validation
│   ├── httpx/                    # JSON helpers, error type, pagination, security middleware
│   ├── auth/                     # passwords, Google sign-in, emailed tokens, sessions, per-account limits (Phase 2)
│   ├── ratelimit/                # in-memory token bucket per key, with refunds; no HTTP
│   ├── admin/                    # `admin create` command; invites, roles (Phase 2); person linking, access requests (Phase 3)
│   ├── family/                   # persons, relationships, tree, ancestors, cycles   (Phase 3)
│   ├── contacts/                 # contact methods, visibility, directory            (Phase 3)
│   ├── event/                    #                                                   (Phase 5)
│   ├── media/                    # upload/confirm/view; worker in Phase 6            (Phase 5)
│   ├── story/                    # stories, chapters, transcripts, captions          (Phase 5b)
│   ├── mail/                     # Mailer interface + net/smtp impl
│   ├── storage/                  # Storage interface + s3.go (presign, head, delete)    (Phase 5)
│   ├── database/
│   │   ├── db.go                 # pgxpool setup
│   │   ├── migrate.go            # embedded golang-migrate runner
│   │   ├── migrations/           # one numbered .up.sql/.down.sql pair per phase, from Phase 2 (§6.2)
│   │   ├── testdata/migrations/  # throwaway migrations for the runner's tests
│   │   ├── seed.sql              # made-up sample family for local dev (`make seed`)
│   │   ├── queries/              # *.sql consumed by sqlc
│   │   └── gen/                  # sqlc output (committed; CI checks it's current)
│   └── webui/
│       ├── embed.go              # //go:embed all:dist  (package webui)
│       ├── handler.go            # SPA handler
│       └── dist/                 # Vite output (git-ignored; stubbed by `make ui-stub`)
│   (each package with interfaces gets a mocks/ subfolder: mockery output, committed)
├── web/                          # React app (Vite outDir -> ../internal/webui/dist)
├── deploy/                       # Caddyfile, compose.prod.yaml, backup + restore scripts (Phase 4c)
├── docs/
│   └── architecture-v3.md        # the design; kept current as decisions change
├── CLAUDE.md                     # working rules (branches, commits, approach)
├── README.md  .gitignore  .env.example
├── compose.yaml                  # postgres, mailpit, storage
└── Dockerfile  .dockerignore  Makefile  sqlc.yaml  .mockery.yml  .golangci.yml  go.mod
```

Folders are created in the phase that first needs them, not up front.

Three layers, dependencies pointing one way only: `router` → `handler` → feature packages (`auth`, `admin`, `family`, `contacts`, `event`, `media`) → `database/gen`. Feature packages hold the services (logic and permission checks) and know nothing about HTTP; `handler` holds all HTTP code; routes live only in `router/` (§6.1). Nothing imports `handler` except `router`, so there are no import cycles.

Why the embed package is not inside `web/`: Go tooling walks `./...`, and `node_modules` can contain stray `.go` files. Keeping Go code out of the Node tree avoids that.

Why `family/` is one package: relationship writes need the ancestor walk (cycle check) and the tree needs both people and relationships; splitting them into `person/relationship/tree` would make them import each other.

## 3. Database schema

Conventions: **Postgres 18**. UUID PKs via the built-in `uuidv7()` (Postgres 18, no extension): time-ordered, so B-tree indexes stay compact. `uuidv7()` embeds the creation time. That's acceptable: IDs reach signed-in members, and media IDs also appear in presigned bucket URLs (§9), which can be forwarded. Those keys already carry the upload year (`media/<yyyy>/...`), and anyone holding the URL can see the photo itself, so the extra exposure is only the upload timestamp. **Postgres 18 everywhere:** local compose, the CI test service, production, and the `pg_dump` client used for backups (an older `pg_dump` refuses a newer server). Managed hosting (§14 Option B) is only an option on providers that offer 18. `TIMESTAMPTZ` everywhere. Partial dates are stored as a date plus a precision flag (`1950-01-01` + `year` means "1950"). Soft delete via `deleted_at` on people, events and media.

### 3.1 Full schema (written as one migration per phase, §6.2)

Phase 2's migration (`000001_create_auth_tables`) holds `users`, `auth_tokens`, `sessions` and `oauth_flows`; `users.person_id`, its unique index and its foreign key arrive with `persons` in Phase 3, as do `pg_trgm` and `masked_birth_date`.

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;   -- fuzzy name search for tagging autocomplete

-- Authentication ------------------------------------------------------------
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT uuidv7(),
  email         TEXT NOT NULL UNIQUE CHECK (email = lower(email)),
  role          TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member')),
  person_id     UUID,                                  -- FK added below
  password_hash TEXT,                                  -- Argon2id PHC string; NULL until set (Google-only is fine)
  password_changed_at TIMESTAMPTZ,
  google_sub    TEXT UNIQUE,                           -- Google account ID, linked on first Google sign-in
  disabled_at   TIMESTAMPTZ,
  last_login_at TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((password_hash IS NULL) = (password_changed_at IS NULL))
);

-- Single-use emailed links: invite (set first password) and password reset
CREATE TABLE auth_tokens (
  id          UUID PRIMARY KEY DEFAULT uuidv7(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose     TEXT NOT NULL CHECK (purpose IN ('invite','password_reset')),
  token_hash  BYTEA NOT NULL UNIQUE,                   -- SHA-256 of a 256-bit random token
  expires_at  TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,                             -- never set: a token is deleted when used (§5); kept, as 000001 shipped, and still checked
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_auth_tokens_user ON auth_tokens(user_id, purpose, created_at DESC);
-- Phase 3 adds who an invite is from (shown in the email and on the Welcome page) and who sent it (audit):
--   ALTER TABLE auth_tokens ADD COLUMN from_person_id  UUID REFERENCES persons(id) ON DELETE SET NULL,
--                           ADD COLUMN sent_by_user_id UUID REFERENCES users(id)   ON DELETE SET NULL;
-- Both stay NULL for password_reset tokens.
CREATE INDEX idx_auth_tokens_expires ON auth_tokens(expires_at);   -- cleanup job

CREATE TABLE sessions (
  id_hash      BYTEA PRIMARY KEY,                      -- SHA-256 of the cookie value
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,                   -- absolute cap
  user_agent   TEXT
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Google sign-in flows, held server-side; the browser only holds an opaque handle (§5)
CREATE TABLE oauth_flows (
  handle_hash   BYTEA PRIMARY KEY,                     -- SHA-256 of the cookie handle
  state         TEXT NOT NULL,
  nonce         TEXT NOT NULL,
  pkce_verifier TEXT NOT NULL,
  expires_at    TIMESTAMPTZ NOT NULL,
  consumed_at   TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_oauth_flows_expires ON oauth_flows(expires_at);

-- People --------------------------------------------------------------------
CREATE TABLE persons (
  id                    UUID PRIMARY KEY DEFAULT uuidv7(),
  first_name            TEXT NOT NULL,
  middle_name           TEXT,
  last_name             TEXT,                           -- optional: older relatives' surnames are often unknown
  maiden_name           TEXT,
  gender                TEXT CHECK (gender IN ('male','female','other','unknown')),
  birth_date            DATE,
  birth_date_precision  TEXT CHECK (birth_date_precision IN ('day','month','year','circa')),
  death_date            DATE,
  death_date_precision  TEXT CHECK (death_date_precision IN ('day','month','year','circa')),
  is_deceased           BOOLEAN NOT NULL DEFAULT false, -- true even when date unknown
  bio                   TEXT,
  search_name           TEXT GENERATED ALWAYS AS (first_name || coalesce(' ' || last_name, '')) STORED,  -- no trailing space; concat_ws isn't immutable so can't be used here
  avatar_media_id       UUID,                           -- FK added below
  created_by            UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at            TIMESTAMPTZ,
  CHECK ((birth_date IS NULL) = (birth_date_precision IS NULL)),
  CHECK ((death_date IS NULL) = (death_date_precision IS NULL)),
  CHECK (death_date IS NULL OR is_deceased),            -- a death date implies deceased
  -- no empty or whitespace-only names: the service trims input and stores a blank surname as NULL,
  -- which also keeps search_name free of a trailing space
  CHECK (btrim(first_name) <> ''),
  CHECK (last_name IS NULL OR btrim(last_name) <> '')
);
-- search_name is a stored generated column, so queries and the index can't drift apart
CREATE INDEX idx_persons_name_trgm ON persons
  USING gin (search_name gin_trgm_ops) WHERE deleted_at IS NULL;

-- Relationships ---------------------------------------------------------------
-- Only two kinds are stored; everything else is derived (§3.4):
-- parent_child : person_a = PARENT, person_b = CHILD  (directional)
--                subtype biological | adoptive | foster   (step-parents are derived, never stored)
-- couple       : symmetric; stored once with person_a < person_b
--                subtype married | partner;  ended = divorced / separated
CREATE TABLE relationships (
  id         UUID PRIMARY KEY DEFAULT uuidv7(),
  type       TEXT NOT NULL CHECK (type IN ('parent_child','couple')),
  subtype    TEXT NOT NULL,
  ended      BOOLEAN NOT NULL DEFAULT false,
  person_a   UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  person_b   UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  notes      TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT rel_distinct  CHECK (person_a <> person_b),
  CONSTRAINT rel_canonical CHECK (type = 'parent_child' OR person_a < person_b),
  CONSTRAINT rel_subtype   CHECK (
       (type = 'parent_child' AND subtype IN ('biological','adoptive','foster'))
    OR (type = 'couple'       AND subtype IN ('married','partner'))),
  CONSTRAINT rel_ended     CHECK (NOT ended OR type = 'couple'),
  CONSTRAINT rel_unique    UNIQUE (type, person_a, person_b)  -- one couple row per pair; one parent link per parent/child
);
CREATE INDEX idx_rel_a ON relationships(person_a);
CREATE INDEX idx_rel_b ON relationships(person_b);

-- Contact info ----------------------------------------------------------------
-- Private by default. A row is shown to other users only when visibility = 'family'.
-- The linked user and admins can always see their own / all rows.
CREATE TABLE contact_methods (
  id         UUID PRIMARY KEY DEFAULT uuidv7(),
  person_id  UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL CHECK (kind IN ('email','phone','address','other')),
  label      TEXT,                                      -- "Mobile", "Home", "Instagram"
  value      TEXT NOT NULL,
  visibility TEXT NOT NULL DEFAULT 'hidden' CHECK (visibility IN ('hidden','family')),
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_contact_methods_person ON contact_methods(person_id);

-- Access requests (Phase 3): someone asks for an invitation, or a member suggests a relative.
-- A request never creates an account; an admin approves it by sending the normal invite (§5).
CREATE TABLE access_requests (
  id              UUID PRIMARY KEY DEFAULT uuidv7(),
  full_name       TEXT NOT NULL CHECK (btrim(full_name) <> ''),
  email           TEXT NOT NULL CHECK (email = lower(email)),
  side            TEXT CHECK (side IN ('first','second','married_in')),  -- labels from FAMILY_SIDES (§6.5)
  relationship    TEXT,                                 -- "daughter", "brother", … from the form's list
  related_names   TEXT[] NOT NULL DEFAULT '{}',         -- names as typed, 1–4
  note            TEXT,
  source          TEXT NOT NULL CHECK (source IN ('self','member_suggestion')),
  suggested_by    UUID REFERENCES users(id) ON DELETE SET NULL,
  status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','declined')),
  reviewed_by     UUID REFERENCES users(id) ON DELETE SET NULL,
  reviewed_at     TIMESTAMPTZ,
  invited_user_id UUID REFERENCES users(id) ON DELETE SET NULL,  -- set when approved and invited
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- not a CHECK pairing source with suggested_by: ON DELETE SET NULL would then fail when the suggester's user is deleted
  CHECK (source = 'member_suggestion' OR suggested_by IS NULL)
);
CREATE INDEX idx_access_requests_pending  ON access_requests(created_at)  WHERE status = 'pending';
CREATE INDEX idx_access_requests_declined ON access_requests(reviewed_at) WHERE status = 'declined';  -- cleanup job

-- Events ------------------------------------------------------------------------
CREATE TABLE events (
  id                   UUID PRIMARY KEY DEFAULT uuidv7(),
  title                TEXT NOT NULL,
  event_date           DATE,
  event_date_precision TEXT CHECK (event_date_precision IN ('day','month','year','circa')),
  location             TEXT,
  description          TEXT,
  created_by           UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at           TIMESTAMPTZ,
  CHECK ((event_date IS NULL) = (event_date_precision IS NULL))
);
CREATE INDEX idx_events_date ON events(event_date DESC NULLS LAST) WHERE deleted_at IS NULL;

CREATE TABLE event_persons (
  event_id  UUID NOT NULL REFERENCES events(id)  ON DELETE CASCADE,
  person_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  PRIMARY KEY (event_id, person_id)
);
CREATE INDEX idx_event_persons_person ON event_persons(person_id);

-- Media ---------------------------------------------------------------------------
CREATE TABLE media (
  id                UUID PRIMARY KEY DEFAULT uuidv7(),
  event_id          UUID REFERENCES events(id) ON DELETE SET NULL,
  uploader_id       UUID REFERENCES users(id)  ON DELETE SET NULL,
  kind              TEXT NOT NULL CHECK (kind IN ('photo','video','audio')),  -- audio for stories (Phase 5b)
  storage_key       TEXT NOT NULL UNIQUE,               -- media/2026/<uuid>/original.<ext>
  original_filename TEXT,                               -- display only, never used in keys
  mime_type         TEXT NOT NULL,
  size_bytes        BIGINT NOT NULL CHECK (size_bytes > 0),
  caption           TEXT,
  status            TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','ready','failed')),
  processing_status TEXT NOT NULL DEFAULT 'none'
                    CHECK (processing_status IN ('none','queued','processing','done','failed')),
  variants          JSONB NOT NULL DEFAULT '{}',        -- {"thumb":"key","poster":"key","mp4_720":"key"}
  width             INT,
  height            INT,
  duration_seconds  INT,
  taken_on          DATE,                               -- scanned photos are often "circa 1965"
  taken_on_precision TEXT CHECK (taken_on_precision IN ('day','month','year','circa')),
  confirmed_at      TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at        TIMESTAMPTZ
);
CREATE INDEX idx_media_event   ON media(event_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_recent  ON media(created_at DESC) WHERE deleted_at IS NULL AND status = 'ready';
CREATE INDEX idx_media_pending ON media(created_at) WHERE status = 'pending';          -- orphan sweep
CREATE INDEX idx_media_queue   ON media(created_at) WHERE processing_status = 'queued'; -- worker

CREATE TABLE media_tags (
  media_id  UUID NOT NULL REFERENCES media(id)   ON DELETE CASCADE,
  person_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  PRIMARY KEY (media_id, person_id)
);
CREATE INDEX idx_media_tags_person ON media_tags(person_id);

-- Stories (Phase 5b): a recorded video or audio story told by a relative (§9.6)
CREATE TABLE stories (
  id                    UUID PRIMARY KEY DEFAULT uuidv7(),
  title                 TEXT NOT NULL CHECK (btrim(title) <> ''),
  teller_person_id      UUID REFERENCES persons(id) ON DELETE SET NULL,
  media_id              UUID REFERENCES media(id)   ON DELETE SET NULL,   -- the video or audio
  status                TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  consent_confirmed_at  TIMESTAMPTZ,                  -- the teller agreed to share it; required to publish
  recorded_on           DATE,
  recorded_on_precision TEXT CHECK (recorded_on_precision IN ('day','month','year','circa')),
  recorded_by           UUID REFERENCES users(id) ON DELETE SET NULL,
  place                 TEXT,
  summary               TEXT,
  language              TEXT,                         -- spoken language: 'ht', 'fr', 'en', 'mixed'
  chapters              JSONB NOT NULL DEFAULT '[]',  -- [{"at": 400, "title": "…"}], seconds from the start
  transcript            JSONB NOT NULL DEFAULT '[]',  -- [{"at": 12, "text": "…"}], typed or pasted by an editor
  topics                TEXT[] NOT NULL DEFAULT '{}',
  search_tsv            TSVECTOR,                     -- title + summary + transcript, set by the service on write
  featured_at           TIMESTAMPTZ,                  -- "Story of the week": the latest featured one
  created_by            UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at            TIMESTAMPTZ,
  CHECK ((recorded_on IS NULL) = (recorded_on_precision IS NULL)),
  CHECK (status <> 'published' OR consent_confirmed_at IS NOT NULL)
);
CREATE INDEX idx_stories_search ON stories USING gin (search_tsv) WHERE deleted_at IS NULL;
CREATE INDEX idx_stories_teller ON stories(teller_person_id) WHERE deleted_at IS NULL;

CREATE TABLE story_persons (            -- tree people mentioned in or appearing in a story
  story_id  UUID NOT NULL REFERENCES stories(id)  ON DELETE CASCADE,
  person_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  PRIMARY KEY (story_id, person_id)
);
CREATE INDEX idx_story_persons_person ON story_persons(person_id);

ALTER TABLE media ADD CONSTRAINT media_taken_pair
  CHECK ((taken_on IS NULL) = (taken_on_precision IS NULL));

-- One login per person (a person may have no login; a login may have no person yet)
CREATE UNIQUE INDEX uq_users_person ON users(person_id) WHERE person_id IS NOT NULL;

-- Deferred FKs (break the creation-order cycles)
ALTER TABLE users   ADD CONSTRAINT fk_users_person   FOREIGN KEY (person_id)       REFERENCES persons(id) ON DELETE SET NULL;
ALTER TABLE persons ADD CONSTRAINT fk_persons_avatar FOREIGN KEY (avatar_media_id) REFERENCES media(id)   ON DELETE SET NULL;

-- Birth-date masking (§5.2). Every query that returns people to a browser calls this;
-- birth_date is never selected directly. Returns ISO 8601, with "--MM-DD" meaning "no year".
CREATE FUNCTION masked_birth_date(p persons, viewer_is_admin boolean, viewer_person_id uuid)
RETURNS text LANGUAGE sql STABLE AS $$
  SELECT CASE
    WHEN p.birth_date IS NULL THEN NULL
    WHEN viewer_is_admin OR p.is_deceased OR p.id = viewer_person_id THEN
      CASE p.birth_date_precision
        WHEN 'day'   THEN to_char(p.birth_date, 'YYYY-MM-DD')
        WHEN 'month' THEN to_char(p.birth_date, 'YYYY-MM')
        ELSE              to_char(p.birth_date, 'YYYY')      -- 'year' and 'circa'
      END
    WHEN p.birth_date_precision = 'day'   THEN to_char(p.birth_date, '--MM-DD')
    WHEN p.birth_date_precision = 'month' THEN to_char(p.birth_date, '--MM')
    ELSE NULL                                                -- living, year/circa only: hidden
  END
$$;
```

**Accidental-delete protection ("someone deletes grandma").** `persons`, `events` and `media` carry `created_by`/`uploader_id`, `updated_at` and `deleted_at`. `DELETE /api/persons/{id}` is admin-only and only sets `deleted_at`; relationships stay intact, so an admin-only `POST /api/persons/{id}/restore` brings her back with her tree position. A hard purge is a separate manual admin command, never an API call. `updated_at` is set explicitly in every sqlc `UPDATE` (`updated_at = now()`) rather than by a trigger, which keeps behavior visible in the query files.

### 3.2 Full schema rollback (each phase's `.down.sql` drops its own part)

```sql
DROP FUNCTION IF EXISTS masked_birth_date(persons, boolean, uuid);
DROP TABLE IF EXISTS story_persons, stories, media_tags, media, event_persons, events, access_requests,
                     contact_methods, relationships, oauth_flows, sessions, auth_tokens, persons, users CASCADE;
```

Down migrations are a local-dev convenience. In production, fix forward with a new migration.

### 3.3 Known modelling limits (accepted for now)

- Relationships carry no start/end dates (marriage, divorce). A couple's `ended` flag says current vs former; that's all the tree needs. Add dates later only if needed.
- A couple who divorce and remarry each other is one row (`rel_unique`); set `ended` back to false. Their history isn't modelled.
- At most two **biological** parents per person. Enforced in the service (inside the relationship-write transaction, §10), since a row-count rule can't be a plain constraint. Adoptive and foster parents are not limited.
- Gender is a free label for display, not used for tree logic.
- Soft-deleted people keep their relationship rows; tree queries filter on `persons.deleted_at IS NULL`.
- Contact visibility has two levels (`hidden`, `family`). Finer audiences ("close family only", per-person allow lists) are deliberately out of scope; see §16.

### 3.4 Derived relationships (never stored)

Only parent links and couples are stored. Everything else is computed from them, client-side for the tree and labels, server-side where a query needs it, so it can never drift out of sync.

**Which parent links count.** A *family parent* is a `biological` or `adoptive` parent link: adoption makes a child a full member of the family, so the two are treated identically for every rule below. `foster` links never create family relations; they only produce the foster labels at the end of the table. (Whether a link is biological or adoptive is shown on the person's profile, not in the sibling label.)

| Relation | Rule |
|---|---|
| **Full siblings** | Their sets of family parents are **identical** and have at least two people. |
| **Half-siblings** | They share at least one family parent, and **each** has a family parent the other lacks. |
| **Siblings (unspecified)** | They share at least one family parent, but neither rule above holds (typically one person's second parent simply isn't recorded yet). Shown as "sibling" rather than guessing full or half. |
| **Step-parent** | A family parent's couple partner who isn't your parent by any link. Labelled "former step-parent" if that couple is `ended`. |
| **Step-siblings** | Children (via family-parent links) of your step-parent who share no family parent with you. |
| **Grandparents, aunts/uncles, cousins** | Walks over family-parent links only (lowest common ancestor). |
| **In-laws** | One couple hop from the above. |
| **Foster parent / foster child** | A `foster` link, labelled as such; never counted as a parent for any rule above. |
| **Foster siblings** | Share a foster parent, or one is a foster child of the other's family parent, and they aren't already siblings by the rules above. |

The ancestry cycle check (§10) still walks **every** parent link, foster included, so no loop of any kind can be stored.

Households (§7) follow the same data: a half-sibling appears under the household of the parent you share, alongside the other parent if one is recorded. The admin "add sibling" flow asks **full or half**; for half, which parent is shared. It then creates only parent links to the person's recorded parents. Siblings are always linked through parents, so if the person has **no** family parent recorded, the flow first asks the admin to add one (a first name such as "Unknown" is enough; it can be filled in later). A "full" sibling of someone with only one recorded parent is linked to that parent and shows as "sibling" until the second parent is added.

## 4. REST API

JSON in/out. Errors: `{"error": {"code": "not_found", "message": "..."}}` with a proper status. Lists are cursor-paginated (`?limit=&cursor=`). Unknown `/api/*` returns a JSON 404, never `index.html`.

**Roles.** The tree is maintained by admins only, because most family members won't be comfortable editing it.
- `admin`: create, edit and soft-delete/restore people; create and edit relationships; manage users and their person links; edit anyone's contact info; delete any event or media.
- `member`: read the tree and see everyone's *visible* contact info; edit **only their own linked person's** profile (bio, avatar) and contact methods with visibility; (once enabled) create events and upload/tag media. Both are **admin-only at first**; setting `MEMBERS_CAN_CONTRIBUTE=true` opens them to members without a code change. Members can delete their own uploads and events.

Server-side checks enforce this (a member's `PATCH /api/persons/{id}` is rejected unless `id` is their own `person_id`, and then only for profile fields); the UI hiding buttons is a convenience, not the control.

**Auth column:** `session` means a signed-in user is required; `admin` additionally requires the admin role. Only the sign-in flow itself works without a session (`no session (sign-in flow)`), and none of those endpoints return family data. Health checks are reachable only from the server itself.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | internal only (blocked at Caddy) | Liveness: process is up |
| GET | `/ready` | internal only (blocked at Caddy) | Readiness: database reachable |
| POST | `/api/auth/login` | no session (sign-in flow), rate-limited | `{email, password}` → sets session cookie (the session the request's old cookie held, if any, is ended in the same transaction) and returns the user as `/me` does; any failure is the same `401 invalid_credentials` (`503 busy` when the hash limit is full or the user's row stays locked) |
| POST | `/api/auth/password/forgot` | no session (sign-in flow), rate-limited | `{email}` → always `202`; reset email sent only if user exists and is enabled |
| POST | `/api/auth/password/reset` | no session (sign-in flow), rate-limited | `{token, password}` (invite or reset link) → sets password, revokes all the user's sessions and other tokens, signs in (ending the old cookie's session, as login does) and returns the user as `/me` does; `400 invalid_token` for a malformed, unknown, used or expired token or a disabled user, `400 validation_error` for a password the rules refuse, `503 busy` as for login |
| POST | `/api/auth/invite/preview` | no session (sign-in flow), rate-limited | `{token}` → `{invitee_first_name, invitee_email, from_name, google_eligible}` for the Welcome page (`google_eligible` is true for `@gmail.com` and `@googlemail.com` only: a Workspace domain is proven only by the ID token's `hd` claim, so those invitees use "Continue with Google" from the sign-in page; `from_name` is null when the invite has no "from" person), which can't GET the token because it lives in the URL fragment (§5). Consumes nothing; the same `400 invalid_token` as `password/reset` for a malformed, unknown, used or expired token. Phase 2 returns the invitee's email and Google eligibility; names arrive with persons in Phase 3 |
| POST | `/api/access-requests` | no session (sign-in flow), rate-limited | `{full_name, email, side, relationship, related_names, note}` → always `202` with the same body, whether or not the email has an account or a pending request. A honeypot field; lengths capped; email normalized. Never creates an account (§5) |
| POST | `/api/auth/password/change` | session | `{current_password, new_password}` → revokes other sessions |
| GET | `/api/auth/google/start` | no session (sign-in flow), rate-limited | Redirects to Google (state, nonce, PKCE) |
| GET | `/api/auth/google/callback` | no session (sign-in flow), rate-limited | Verifies the ID token; signs in by `sub`, auto-links only Gmail/Workspace invited emails (§5); redirects into the app |
| POST | `/api/admin/users/{id}/send-reset` | admin | Resend invite or send a password-reset email |
| GET | `/api/auth/me` | session | Current user, role, linked person |
| POST | `/api/auth/logout` | none needed | Always clears the cookie, so an expired session's cookie still goes away; deletes the cookie's session row if there is one (`204`, or `500` if that delete failed) |
| GET | `/api/admin/users` | admin | List users |
| POST | `/api/admin/users` | admin | Invite: `{email, role}`; from Phase 3 also `person_id` (required, §5.1), `from_person_id` (optional; a person linked to an active user; defaults to the suggesting member's person, else the sending admin's) and `access_request_id` (optional; marks that request `approved` in the same transaction) |
| GET | `/api/admin/access-requests` | admin | Pending first. Each `related_names` entry comes with its best trigram match among non-deleted persons, so the UI can show "In the tree" or "Not found — check spelling". Matching happens only here, never for the public form |
| POST | `/api/admin/access-requests/{id}/decline` | admin | Quietly sets `declined`; no email is sent |
| POST | `/api/admin/access-requests/{id}/reopen` | admin | Undo a decline |
| POST | `/api/me/suggestions` | session | A member suggests a relative: an access request with `source = member_suggestion`, `suggested_by` = the member |
| PATCH | `/api/admin/users/{id}` | admin | Role, disable/enable, link person, change email (clears Google link and password, revokes sessions and tokens, re-invites; §5) |
| GET | `/api/tree` | session | All non-deleted people + relationships (flat; see §10). No dates and no contacts; each person carries a short `bio_excerpt` for the quick look |
| GET | `/api/persons` | session | `?q=` trigram search for autocomplete |
| POST | `/api/persons` | admin | Create |
| GET | `/api/persons/{id}` | session | Profile, direct relations, tagged media, and **only the contact methods the viewer may see** (§5.2) |
| PATCH | `/api/persons/{id}` | admin, or the linked user for their own profile fields | Update (members: bio and avatar only) |
| DELETE | `/api/persons/{id}` | admin | Soft delete |
| POST | `/api/persons/{id}/restore` | admin | Undo soft delete |
| GET | `/api/persons/{id}/ancestors` | session | `?depth=` (default 5) |
| GET | `/api/persons/{id}/descendants` | session | `?depth=` |
| GET | `/api/me/contacts` | session | Own contact methods with their visibility |
| POST | `/api/me/contacts` | session | Add `{kind, label, value, visibility}` (default `hidden`) |
| PATCH/DELETE | `/api/me/contacts/{cid}` | session (own) | Edit value or toggle visibility; remove |
| GET/POST | `/api/persons/{id}/contacts` | admin | List all / add on behalf of someone (e.g. a relative without a login) |
| PATCH/DELETE | `/api/persons/{id}/contacts/{cid}` | admin | Edit / remove any contact method |
| GET | `/api/directory` | session | `?q=` people with at least one `family`-visible contact method, with only those methods |
| POST | `/api/relationships` | admin | `{type:"parent_child", parent_id, child_id, subtype}` or `{type:"couple", person_ids:[id1,id2], subtype, ended}`. `person_ids` must hold **exactly two distinct** existing, non-deleted people, in any order (the service sorts them). `subtype` is optional for `parent_child` (defaults to `biological`) and required for `couple` (`married` or `partner`, no guessing). Invalid input is a `400 validation_error`, never a database error. Validates direction, duplicates, ancestry cycles, at most two biological parents |
| PATCH | `/api/relationships/{id}` | admin | Subtype, `ended`, notes. Type and people can't change (delete and re-create instead). A subtype change runs the same locked validation as POST, so switching a link to `biological` can't create a third biological parent |
| DELETE | `/api/relationships/{id}` | admin | Hard delete |
| GET | `/api/events` | session | Chronological feed |
| POST | `/api/events` | admin (members when `MEMBERS_CAN_CONTRIBUTE=true`) | Create (with `person_ids`) |
| GET/PATCH | `/api/events/{id}` | GET: session; PATCH: admin, or the creator when contributions are enabled | Read / update |
| DELETE | `/api/events/{id}` | owner/admin | Soft delete |
| POST | `/api/media/upload-url` | admin (members when `MEMBERS_CAN_CONTRIBUTE=true`) | `{filename, mime_type, size_bytes, event_id?}` → `{media_id, url, headers}` |
| POST | `/api/media/{id}/confirm` | uploader | `HeadObject` verify → `ready`; `{person_ids, caption, taken_on, taken_on_precision, width, height, duration_seconds}` (dimensions measured client-side; the worker overwrites them in Phase 6) |
| GET | `/api/media` | session | `?event_id=&person_id=&cursor=` with short-lived thumbnail URLs |
| GET | `/api/media/{id}` | session | Metadata |
| GET | `/api/media/{id}/url` | session | `?variant=original\|poster\|mp4_720` → presigned GET (supports `Range`) |
| PATCH | `/api/media/{id}` | uploader/admin | Caption, tags |
| DELETE | `/api/media/{id}` | uploader/admin | Soft delete (object removal is deferred, §9) |
| GET | `/api/stories` | session | Published stories; `?teller_id=&topic=&q=` (`q` searches title, summary and transcript); the featured one first |
| POST | `/api/stories` | admin (members when `MEMBERS_CAN_CONTRIBUTE=true`) | Create a draft (with `person_ids`, `media_id`) |
| GET/PATCH | `/api/stories/{id}` | GET: session (drafts: their creator and admins); PATCH: admin, or the creator when contributions are enabled | Read / update, including chapters, transcript and publishing (requires `consent_confirmed_at`) |
| DELETE | `/api/stories/{id}` | creator/admin | Soft delete |
| GET | `/api/stories/{id}/captions.vtt` | session | WebVTT captions generated from the transcript |

## 5. Authentication & sessions

**Model: invite-only.** Only emails that an admin has created in `users` can sign in, by either method below. Nobody can self-register. Anyone may **ask** for an invitation (`POST /api/access-requests`), and members can suggest a relative; a request never creates an account. It waits in the admins' Requests queue, which shows whether the names given match people in the tree. Approving it means sending the normal invite; declining is silent. The form always answers the same way, so it reveals nothing about who is a member. The **email address is the username**: one identifier to remember, and it's already needed for invites and resets.

Two ways to sign in, both ending in the same server-side session:

| Method | For whom |
|---|---|
| **Email + password** | Anyone. Password set from the invite email; reset by email. |
| **Continue with Google** | Invited **Gmail and Google Workspace** addresses only; links on first use, no password needed. Everyone else uses their password. Hidden when `GOOGLE_CLIENT_ID` isn't set (e.g. local dev). |

A user may have a password, a linked Google account, or both (`password_hash` and `google_sub` are each optional).

**Email links (invite and password reset).** Both use the same single-use token mechanism, stored in `auth_tokens` with a `purpose`:

1. Generate 32 random bytes; store `SHA-256(token)` (TTL: **7 days** for `invite`, **30 min** for `password_reset`).
   - **A new reset request does not cancel earlier reset links.** `password/forgot` is public, so if it did, anyone who knows an address could keep cancelling the link its owner just received. Each unexpired reset link stays usable until it expires or a credential changes; issuance is capped by the rate limits below (at most 5 per hour per address).
   - A new **invite** (admin-only) does replace the previous unconsumed invite.
   - **Any credential change invalidates all of the user's outstanding tokens** (every purpose): consuming a token, setting or changing a password, linking Google, an admin changing the email, and disabling the user. An old invite or reset link therefore never outlives the credential it was meant to create.
2. Email `https://<BASE_URL>/auth/set-password#t=<token>`. The token is in the **URL fragment**, so it never reaches server logs or `Referer`, and email link-scanners that GET links can't burn it. The SPA page shows a "choose a password" form and POSTs `{token, password}` to `/api/auth/password/reset`.
3. **All-or-nothing, in one transaction:** look up the token, then lock the user's row (`SELECT … FROM users WHERE id = $1 FOR UPDATE`) and, under that lock, re-check that the token still exists and is unexpired and the user is enabled and still has the email the token was looked up with. The password is checked and hashed before the lock (Argon2 is slow), but only after the token is found, so a malformed or unknown token never costs a hash. A lock that can't be had within the 5 s `lock_timeout`, or a deadlock, is `503 busy`. Then consume the token by deleting it with all the user's other tokens, set the password, delete all sessions, create the new session, and commit. Any failure rolls everything back, so recovery is never half-applied.

**One lock for every credential change, and for every new session.** Token consumption, token issuance, password set/change, Google linking, admin email change, disabling, **and every successful sign-in** take the same `FOR UPDATE` lock on the user's row first and re-check eligibility under it. Concurrent requests for the same person run one after another instead of interleaving.

- **Password sign-in:** Argon2 verification runs *before* the lock (it's slow). Then, inside the transaction, lock the row and confirm the user is still enabled, still has the email signed in with, and `password_changed_at` is unchanged since the verify. If anything changed meanwhile (a reset or admin email change won the race), fail with the usual `401`. A rehash doesn't touch `password_changed_at`, so two logins at once don't refuse each other. The lock wait is capped by a transaction-local `lock_timeout` of 5 s (then `503 busy`, as is a deadlock or that timeout on any later statement in the transaction), so a held row can't pin pool connections. Only then write a rehash (if parameters changed; computed before the lock, and skipped with a warning if it fails) and create the session, so a stale login can never resurrect a revoked password or overwrite a new one.
- **Google sign-in by `sub`:** inside the transaction, lock the row and confirm the user is enabled and still has that `google_sub` before creating the session, so a concurrent email change or disable can't be raced. A flow started before an email change can't re-link either: the `sub` was cleared, and Google's (old) email no longer matches `users.email`.

A 256-bit random token makes brute force moot; plain SHA-256 is appropriate for high-entropy secrets.

**Rules for every sign-in path**

- **Emails are normalized** (trimmed, lowercased) wherever one enters the system: login, forgot password, invites, admin edits and the email Google returns. `users.email` is stored lowercase, so comparisons are plain equality.
- **A disabled user can't sign in by any path** (password, emailed token, Google by `sub` or first link), and disabling deletes their sessions and tokens.

**Email + password sign-in**

- `POST /api/auth/login {email, password}`. On failure, always the same `401 invalid_credentials`, whether the email is unknown, the password is wrong, the user has no password (Google-only) or is disabled. **Every failure path runs exactly one Argon2 verification** (against a dummy hash built with the *current* parameters when there's no usable password), so response time doesn't reveal which emails have accounts or how they sign in. **Residual leak, accepted:** after the parameters are raised, stored hashes that haven't been re-hashed yet still verify at the old cost, so for that account a wrong password answers measurably faster or slower than an unknown email. It only exists during a parameter migration, only reveals "this email has a password", and closes as people sign in. Don't raise parameters casually.
- **Hashing:** Argon2id (`golang.org/x/crypto/argon2`), stored as a PHC string (`$argon2id$v=19$m=…,t=…,p=…$salt$hash`) with a 16-byte random salt. Parameters start at OWASP's minimum (m=19 MiB, t=2, p=1; checked against the Password Storage Cheat Sheet, Nov 2026); because they're stored per hash, they can be raised later and old hashes are re-hashed on the next successful login. At most 4 hashes run at once (a semaphore); excess requests wait briefly or get `503`, so a burst of password attempts can't exhaust the VPS's memory.
- **Password rules:** minimum 8 characters, maximum 128, counted as Unicode characters, no composition rules (no "must include a symbol"). Passwords are NFC-normalized (`golang.org/x/text`) before checking and hashing, so an accented password typed on another device still matches. A new password is refused if it's on the embedded list of common passwords (the NCSC's 100,000 most-used, via SecLists, 8+ characters; matched case-insensitively), or is the user's email, its name part, or the app's name. Long passphrases are encouraged in the UI. NIST SP 800-63B-4 asks for 15 characters when a password is the only factor; 8 is the owner's choice for a family audience, with the blocklist and rate limits as the counterweight.
- **Forgot password:** `POST /api/auth/password/forgot {email}` always returns `202` with the same body (no enumeration) and sends the reset email asynchronously, only if the user exists and is enabled. Google-only users get the same email, which lets them add a password.
- **Change password** (signed in): `POST /api/auth/password/change {current_password, new_password}`; signs out every other session. `current_password` is always required (a session cookie alone may be old or stolen); wrong attempts run one Argon2 verify each and are limited per user (5 per 15 min).

**Continue with Google** (OpenID Connect, authorization-code flow with PKCE; `golang.org/x/oauth2` + `github.com/coreos/go-oidc/v3`). Sign-in only; there is no self-service linking of Google to an existing account.

1. `GET /api/auth/google/start` creates a row in **`oauth_flows`** (server-side; §3.1) holding `state`, `nonce`, the PKCE verifier and a 10-minute expiry. The browser only gets an **opaque random handle** in a flow cookie (`fh_oauth`; `HttpOnly; Secure; SameSite=Lax; Path=/api/auth/google`). Then it redirects to Google.
2. `GET /api/auth/google/callback` looks up the flow by the handle's hash and **consumes it atomically** (single use, unexpired), checks `state`, exchanges the code, and verifies the ID token (signature, issuer, audience, expiry, `nonce`). Nothing about the flow can be forged or replayed from the client side, because none of it lives there.
3. Matching, never creating:
   - If an **enabled** user already has this `google_sub`, sign them in.
   - Otherwise, **automatic linking by email** happens only where Google is authoritative for the mailbox: the (normalized) email is `@gmail.com` / `@googlemail.com`, or the ID token carries a Workspace `hd` claim matching the email's domain, **and** `email_verified` is true, **and** it matches an invited, enabled user with no `google_sub` yet. Then link the `sub` and sign them in. For any other address (e.g. a Yahoo address on a Google account), `email_verified` may reflect an old verification of a mailbox that has since changed hands, so it's not proof.
   - Otherwise show a generic "no invitation found for this account" page. Nothing is created. Relatives whose invited address isn't Gmail or Workspace simply use their password; the page says so.
4. After linking, sign-in is by `sub`, so a later change to the Google account's email doesn't lock them out. A Google link is removed only as part of an admin email change (below).

**Invites.** Sending an invite is admin-only. The admin invites `{email, role}` (plus `person_id` from Phase 3, §5.1), and the invite names who it is **from**: the member who suggested the person, or otherwise the sending admin (`from_person_id`; the token row also keeps who sent it). Subject *"<From first name> invited you to the family hub"* (or *"You're invited to the family hub"* when there is no "from" person, e.g. an admin made with `admin create` and not yet linked); the email adds *"Approved by <admin>, admin."* when the two differ, and says *"Set a password, or sign in with Google if this is your Google address."* The Welcome page shows the same names through `POST /api/auth/invite/preview`. Admins can resend an invite or trigger a password reset from the users screen. Exact copy: `docs/design/design-notes.md` §4.10.

**When an admin changes a user's email** (e.g. fixing a typo'd invite): the user's `google_sub` **and password** (`password_hash`, `password_changed_at`) are cleared, their sessions and outstanding tokens are deleted, and a fresh invite goes to the new address. Whoever received the old address loses all access, since the email itself isn't a secret and the old password must not keep working. The real person sets a new password from the fresh invite, or re-links Google. There is deliberately **no standalone "unlink Google"**: Google only auto-links addresses it owns (Gmail/Workspace), so an unlinked account would simply re-link on its next sign-in. An unwanted link can only come from a wrong email address, which an email change fixes (the old Google account no longer matches); disabling the user cuts off all access.

**Sessions**

- Cookie value: 32 random bytes, base64url. DB stores `SHA-256` of it (`sessions.id_hash`).
- Cookie flags: `HttpOnly; Secure; SameSite=Lax; Path=/`. Name `__Host-fh_session` in prod. With `COOKIE_SECURE=false` (set by `make dev-api` and `make run`) the cookie is `fh_session` without `Secure`: Safari doesn't accept `Secure` cookies on `http://localhost` (verified 2026-11: WebKit bugs 218980/232088 are still open), and the `__Host-` prefix requires `Secure`.
- Lifetime: 30-day sliding window, 180-day absolute cap. `last_seen_at` is only rewritten when it's more than an hour stale, to avoid a DB write on every request. Logout and "disable user" delete rows, so revocation is immediate.

**Auth cleanup job** (Phase 2, in-process, no extra infrastructure): a ticker runs every 15 minutes and deletes, in batches of 1,000, `oauth_flows` that are consumed or expired, `auth_tokens` expired more than 7 days ago (a used token is already deleted), `sessions` past their expiry, and (from Phase 3) `access_requests` declined more than 90 days ago. Expiry is already enforced on every read; the job only keeps the tables from growing. Each table has an index on its expiry column for this.

**Rate limits** (in memory, fine for a single instance). `internal/ratelimit` is a small token bucket per key rather than `x/time/rate`, which can't give back a token granted at once; it drops a bucket only once it has refilled, so memory stays bounded and no penalty is forgiven early. Each limit is a bucket of N that refills evenly over the window ("5 per 15 min" is 5 at once, then one more every 3 minutes), so a sustained attacker gets N per window after the first burst. A blocked request gets `429 rate_limited` with `Retry-After`. Each endpoint's limits land with the endpoint. Two layers:

- **Per IP, as router middleware** (it only needs the client address; IPv6 is counted per /64, since one household usually holds a whole /64): `login` 20 per 15 min, `password/forgot` 20/hour, `password/reset` and `invite/preview` 10/minute each, `access-requests` 20/hour, `google/*` 20/minute.
- **Per account, inside the auth service** (only the service knows the email and whether verification failed), counting **failures only**:
  - `login`: 5 failures per 15 min per **email + IP** pair, and a backstop of 50 failures per hour per email across all IPs. Each attempt takes its slot *before* the Argon2 verify (so parallel guesses can't all pass an unspent limit) and gets it back unless the password was wrong; a right password refused because a credential changed mid-login isn't a failure. Emails over 254 characters are refused before they become limiter keys.
  - `password/change`: 5 wrong `current_password` attempts per 15 min per user.
  - `password/forgot`: 5 emails per hour per address.
- **Per address, in the access-request service** (every request counts, not just failures): 5 per hour per email; over the limit it still answers `202` and stores nothing.

Honest limits of this: a determined attacker rotating IPs can, at worst, hit the 50/hour backstop and **delay password sign-in** for one account. They can never block that person's **Google sign-in or password reset**, both of which prove ownership another way, and the login error page links to both. There's no hard lockout.

**CSRF:** `SameSite=Lax` plus the stdlib `http.CrossOriginProtection` (Go 1.25), which rejects non-safe cross-origin browser requests using `Sec-Fetch-Site` and `Origin`; and require `Content-Type: application/json` when a body exists. **(verify it accepts the Vite-proxied dev origin without extra config; otherwise add `BASE_URL` with `AddTrustedOrigin`.)**

**Bootstrap:** `family-hub admin create --email you@example.com` creates the first admin and **prompts for the password on the terminal** with hidden input (`golang.org/x/term`), twice, checked against the password rules. It refuses an email that already belongs to a user rather than changing that account. It also works once admins exist, as the way back in while mail isn't set up; whoever can run it already has the database credentials. Nothing secret is printed, so nothing lands in Docker or journald logs, and the very first sign-in needs no working mail setup. The command refuses to run without an interactive terminal (no env-var backdoor).

**Google callback and CSRF:** the callback is a top-level `GET` redirect from Google, which `SameSite=Lax` cookies and `CrossOriginProtection` both allow (it only blocks unsafe methods); the `state` check is what protects it.

### 5.1 Linking a login to a person (so people land in the right spot)

An admin decides who each login *is*; nobody claims a spot themselves. This lands in Phase 3 with the `persons` table; until then, invited logins are unlinked.

- **Invite flow:** the admin picks the person first (an approved request opens this step with its details filled in) (trigram search over the tree), or creates the person in the same step, then enters the email and role. Searching before creating prevents duplicate nodes. From Phase 3, a person is **required**: every invited login is linked at invite time, and logins invited before then are linked from the admin screen. (Only admins made with `admin create` may start unlinked; they link themselves from the admin screen.)
- **Constraint:** `uq_users_person` allows at most one login per person. Couples who share an email address still need separate invites.
- **Guard:** soft-deleting a person who has an active login is refused until the admin unlinks or disables that login.
- **On login:** `/api/auth/me` returns `person_id`; the tree opens centered on that person with a "you" marker and a "back to me" button, and relationships are labelled from their point of view ("your aunt Rosa"). Labels are computed client-side from the already-loaded tree using the lowest common ancestor; build that as a pure function with a table-driven test (cousins once removed, half-siblings, step-relations).
- **Email changes:** admin edits `users.email`; the person link, and therefore tree position, is unaffected.

### 5.2 Contact info and visibility

Each person's contact details live in `contact_methods`, one row per phone/email/address/other, each with its own visibility.

| Rule | Behavior |
|---|---|
| Default | `hidden`. Nothing is shared until the person flips a toggle ("Visible to family"). |
| Phones and addresses | Never shown to other members (the design lists them only "on request", which is deferred, §16): the service refuses `family` for `phone` and `address` rows, so they stay with the owner and admins. Emails and `other` rows can be `family`. |
| Who edits | The linked user edits their own rows via `/api/me/contacts`; admins can edit anyone's, including relatives with no login (admin-entered rows also default to `hidden`; the admin marks them `family` only with that person's consent). |
| Who sees a row | The owner and admins: always. Any other signed-in user: only rows with `visibility = 'family'`. |
| Enforcement | In the SQL/service layer (`WHERE visibility = 'family'` for non-owners), never in the frontend. Hidden values are not sent to the browser at all. |
| Where contacts appear | `GET /api/persons/{id}` and `GET /api/directory` only. They are **not** in `/api/tree`, `/api/persons?q=` search results, logs, or error messages, and search never matches on contact values. |
| Birthdays | **Month and day** are visible to every signed-in user. The birth **year** of a *living* person is visible only to admins and to that person themselves; for deceased people the full date is visible to all. Masking happens in SQL via the `masked_birth_date` function (§3.1), so the year never reaches a non-permitted browser. `/api/tree` carries no dates at all (§10). A living person with only a year or "circa" precision shows nothing to others. |
| Login email | `users.email` is an authentication detail and is **never** exposed. A contact email is a separate, optional row the person chooses to add. |
| Admin visibility | Admins can see everything, and the profile settings page says so ("Administrators can see all information"). |
| First login | A short onboarding step walks the user through their profile and shows each contact field with its visibility toggle, defaulting to off. |

Changes take effect immediately because nothing about contacts is cached client-side across sessions.

**Be upfront about admin access.** Hidden contact info is hidden from other *family members*, not from whoever runs the system: the admin account and anyone with database or backup access can read it. That's an honest and reasonable trade-off for a family project, but say so in the UI and the invite email ("Administrators can see all information you enter"). Lighter protection options, if you ever want them: keep the DB and backup buckets restricted to you, and never log contact values. Application-level encryption of address/phone values is possible but only protects against a stolen backup, not against the admin, so I'd skip it for now.

### 5.3 Upcoming birthdays (in-app only)

Birthdays are shown in the app; **no birthday emails are sent** (decided; see §16). The only emails the app sends are invites and password resets.

- **Panel:** the home page shows living, non-deleted people with `birth_date_precision = 'day'` whose birthday falls in the next ~14 days, relative to "today" in `APP_TIMEZONE`. Feb 29 birthdays show on Feb 28 in non-leap years.
- **Masking:** same rules as everywhere (`masked_birth_date`, §3.1), so members see "Rosa, Friday" and never her age.
- **Where it lives:** one query in the `family` package; no scheduled job, no extra tables.
- **Mail deliverability (for invites and password resets):** sending from a domain you control needs SPF, DKIM and DMARC records with your mail provider; set that up in Phase 4c and send a test to Gmail and Outlook before relying on it. Volume is tiny, so any provider's free tier works.

### 5.4 People without logins (curated profiles)

Many relatives, especially older ones, will never have an account, and the tree is complete without them.

- A `persons` row needs no `users` row. It can have a bio, photos tagged to it, events, and (admin-entered) contact methods, all maintained by the admin.
- Everything about them is visible according to the same rules: dates and bio to all signed-in users, contact methods only if the admin marks them `family` with that person's consent.
- If a relative gets a login later, the admin just links the existing person to the new user (§5.1); nothing is re-entered and their photos and tags stay attached.
- **Collection tracker (admin view):** a simple list of people missing a photo, birth date, or bio, so you can work through "who do I still need to call?" Cheap to build (one query) and useful when chasing relatives. Add to Phase 5.
- **Where info came from:** `persons.bio` holds the story, including any uncertainty ("Aunt Rosa says 1948").

## 6. Backend wiring

### 6.1 Router and handlers

**Routing, handling and logic are separate packages.**

- `internal/router`: the only place routes and middleware are wired. One file shows every endpoint, its method and the auth it requires, matching the §4 table line for line.
- `internal/handler`: all HTTP code, one file per area (`auth.go`, `family.go`, ...). Decode the request, call a service, encode the response. No SQL, no business rules.
- Feature packages (`internal/family`, `internal/contacts`, ...): the services, with business logic and permission checks; they talk to sqlc queries and know nothing about HTTP.
- Each handler depends on a service **interface declared in `handler`** (the consumer), e.g. `handler.FamilyService`. mockery generates mocks of those into `internal/handler/mocks/` for handler tests.

`internal/app` builds the dependencies (pool, services, handlers) and passes them to `router.New`.

```go
// internal/handler/family.go
package handler

type FamilyService interface {
    Tree(ctx context.Context, viewer auth.Viewer) (family.Tree, error)
    GetPerson(ctx context.Context, viewer auth.Viewer, id uuid.UUID) (family.Person, error)
    // ...
}

type Family struct{ svc FamilyService }

func NewFamily(svc FamilyService) *Family { return &Family{svc: svc} }

func (h *Family) Tree(w http.ResponseWriter, r *http.Request) { /* decode, call, encode */ }
```

```go
// internal/router/router.go
package router

type Handlers struct {
    Health   *handler.Health
    Auth     *handler.Auth
    Admin    *handler.Admin
    Family   *handler.Family
    Contacts *handler.Contacts
    Event    *handler.Event   // Phase 5
    Media    *handler.Media   // Phase 5
}

type Middleware struct {
    RequireSession func(http.Handler) http.Handler // handler.SessionGate.Require; the auth service has no HTTP
    // per-IP limits only; per-account failure limits live in the auth service (§5)
    LimitLogin     func(http.Handler) http.Handler
    LimitForgot    func(http.Handler) http.Handler
    LimitReset     func(http.Handler) http.Handler
    LimitGoogle    func(http.Handler) http.Handler
    TrustedProxy   bool
}

func New(h Handlers, mw Middleware) http.Handler {
    r := chi.NewRouter()
    r.Use(httpx.RequestID)              // a fresh ID per request; the client's is ignored
    if mw.TrustedProxy {                // only when actually behind a proxy you control
        r.Use(httpx.ClientIPFromProxy)  // last X-Forwarded-For entry; chi's RealIP prefers headers a client can forge
    }
    r.Use(httpx.SecurityHeaders)        // CSP, nosniff, Referrer-Policy, X-Robots-Tag: noindex
    r.Use(httpx.RequestLogger("/health", "/ready")) // zerolog JSON; successful health probes left out
    r.Use(httpx.Recover)                // logs the panic and its stack, answers 500 with only the headers above
    r.Use(middleware.GetHead)           // HEAD answers like GET, for uptime monitors

    r.Get("/health", h.Health.Live)
    r.Get("/ready", h.Health.Ready)

    r.Route("/api", func(r chi.Router) {
        r.Use(http.NewCrossOriginProtection().Handler) // stdlib CSRF (Go 1.25)
        r.NotFound(httpx.NotFound)
        r.MethodNotAllowed(httpx.MethodNotAllowed)

        r.With(mw.LimitLogin).Post("/auth/login", h.Auth.Login)
        r.With(mw.LimitForgot).Post("/auth/password/forgot", h.Auth.ForgotPassword)
        r.With(mw.LimitReset).Post("/auth/password/reset", h.Auth.ResetPassword)
        r.With(mw.LimitGoogle).Get("/auth/google/start", h.Auth.GoogleStart)
        r.With(mw.LimitGoogle).Get("/auth/google/callback", h.Auth.GoogleCallback)
        r.Post("/auth/logout", h.Auth.Logout)           // no session needed: always clears the cookie

        r.Group(func(r chi.Router) {
            r.Use(mw.RequireSession)
            r.Get("/auth/me", h.Auth.Me)
            r.Post("/auth/password/change", h.Auth.ChangePassword)

            r.Get("/tree", h.Family.Tree)
            r.Get("/persons", h.Family.SearchPersons)
            r.Get("/persons/{id}", h.Family.GetPerson)
            r.Patch("/persons/{id}", h.Family.UpdatePerson)   // admin, or own profile fields (checked in service)
            r.Get("/persons/{id}/ancestors", h.Family.Ancestors)
            r.Get("/persons/{id}/descendants", h.Family.Descendants)
            r.Get("/directory", h.Contacts.Directory)
            r.Route("/me/contacts", func(r chi.Router) {
                r.Get("/", h.Contacts.ListMine)
                r.Post("/", h.Contacts.CreateMine)
                r.Patch("/{cid}", h.Contacts.UpdateMine)
                r.Delete("/{cid}", h.Contacts.DeleteMine)
            })

            r.Group(func(r chi.Router) {             // admin-only writes
                r.Use(auth.RequireRole("admin"))
                r.Post("/persons", h.Family.CreatePerson)
                r.Delete("/persons/{id}", h.Family.DeletePerson)
                r.Post("/persons/{id}/restore", h.Family.RestorePerson)
                r.Post("/relationships", h.Family.CreateRelationship)
                r.Patch("/relationships/{id}", h.Family.UpdateRelationship)
                r.Delete("/relationships/{id}", h.Family.DeleteRelationship)
                r.Get("/persons/{id}/contacts", h.Contacts.ListFor)
                r.Post("/persons/{id}/contacts", h.Contacts.CreateFor)
                r.Patch("/persons/{id}/contacts/{cid}", h.Contacts.UpdateFor)
                r.Delete("/persons/{id}/contacts/{cid}", h.Contacts.DeleteFor)
                r.Get("/admin/users", h.Admin.ListUsers)
                r.Post("/admin/users", h.Admin.InviteUser)
                r.Patch("/admin/users/{id}", h.Admin.UpdateUser)
                r.Post("/admin/users/{id}/send-reset", h.Admin.SendReset)
            })
            // Phase 5: /events and /media routes added here the same way.
        })
    })

    r.Get("/*", webui.Handler().ServeHTTP) // SPA fallback, mounted last; /api/* never reaches it
    return r
}
```

Role checks in the router (`RequireRole`) are the coarse gate; the fine-grained rules (a member may PATCH only their own person, only profile fields) live in the service, so they're enforced and tested regardless of routing.

Changes vs V2: no CORS middleware (Vite proxies `/api`, so dev is same-origin too); no compression middleware (Caddy compresses in prod); no per-request timeout middleware yet (the `http.Server` timeouts bound each connection); CSRF via the stdlib instead of a hand-written guard; JSON 404s; security headers; a JSON decode helper (`httpx.DecodeJSON`) that requires `Content-Type: application/json` (415 otherwise), limits the body to 1 MiB via `http.MaxBytesReader` (413), and rejects unknown fields and trailing data (400).

### 6.2 Embedded migrations (`internal/database/migrate.go`)

`database.MigrateUp(ctx, url)`, used by `family-hub migrate up` and by `serve` with `AUTO_MIGRATE`, runs the migrations embedded from `internal/database/migrations/` through golang-migrate's `iofs` source and pgx v5 driver. We open the connection with pgx and hand it to the driver, because the driver's own URL handling re-encodes the query (`%20` becomes `+`, which pgx reads literally, so a password with a space fails). Two timeouts are appended to the URL unless it sets them, without re-encoding the rest (that's why `DATABASE_URL` must be a URL, §6.5) (both are constants at the top of `migrate.go`): `connect_timeout` of 10 seconds, so an unreachable host fails fast, and `lock_timeout` of 5 minutes, because the driver waits for its advisory lock with no deadline, so a second start waits its turn, then fails rather than hanging. golang-migrate's own lock wait (default 15 seconds) is set to 24 hours, so Postgres always ends the wait first, even when the URL raises `lock_timeout`. `lock_timeout` also covers every statement in a migration, so a migration that alters an existing table wraps itself in `BEGIN; SET LOCAL lock_timeout = '5s'; … COMMIT;` (a plain `SET` would carry over to later migrations in the run) rather than queueing app queries behind it for minutes. It refuses a dirty database and one at a version the binary doesn't have (an older release); golang-migrate makes those checks under the lock, so concurrent starts are safe, which is why there are no pre-checks of our own. It applies one migration at a time (`Steps(1)`, each under the lock) and checks `ctx` between them, so a canceled run stops cleanly between migrations, never inside one; golang-migrate's own `GracefulStop` is a data race. The lock wait itself doesn't watch `ctx`, so a second Ctrl-C (or Docker's SIGKILL) ends it. With no migrations embedded it returns without connecting.

**No `migrate down`.** Rolling back a schema on real data loses data, so a change is undone by a new migration (or a backup restore). Locally, reset with `docker compose down -v`, `make up`, `make migrate-up`. Each migration still ships a `.down.sql`: a test applies every embedded migration, rolls them all back and applies them again.

**One migration per phase:** each phase adds the tables its code uses, starting with the auth tables in Phase 2. §3.1 shows the full schema as one file for reading; it's split by phase when written.

**Tests** run against `TEST_DATABASE_URL`, each in a throwaway schema (`search_path`), using small migrations in `internal/database/testdata/`; the mail tests send to Mailpit and read the result back through its API at `TEST_MAILPIT_URL`. `make test` always sets both, so run `make up` first; a plain `go test` without them skips those tests locally, and CI (which runs Postgres and Mailpit as services) fails instead of skipping.

### 6.3 SPA handler (`internal/webui`)

```go
//go:embed all:dist
var dist embed.FS

func handler(files fs.FS) http.Handler {          // Handler() passes fs.Sub(dist, "dist")
    fileServer := http.FileServerFS(files)
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
        r = r.Clone(r.Context())
        r.URL.Path = "/" + name                          // net/http refuses a path with ".."
        info, err := fs.Stat(files, name)
        if err != nil && path.Ext(name) != "" {          // missing file, e.g. a stale tab's old asset
            http.NotFound(w, r)
            return
        }
        if err != nil || info.IsDir() {
            w.Header().Set("Cache-Control", "no-cache")  // root, client-side routes, directories
            http.ServeFileFS(w, r, files, "index.html")
            return
        }
        if strings.HasPrefix(name, "assets/") {          // Vite fingerprinted files
            w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
        }
        fileServer.ServeHTTP(w, r)
    })
}
```

The root and every path that isn't a file get `index.html`, never cached, so a deploy shows up on the next load. A directory gets it too, so no directory is ever listed. A missing path with a file extension is a plain 404 instead, so a stale tab loading an old asset fails clearly rather than getting HTML. The cleaned path is what gets served, since `net/http` refuses one containing `..`. The router mounts it last as `GET /*`, after an `/api` group whose `NotFound` is a JSON 404, so an unknown `/api` path never gets `index.html`. `all:` is required so a placeholder dotfile would still embed; the stub approach (§8.1) avoids needing one.

### 6.4 `main.go` shape

The binary has only the commands production needs; dev-only tasks live in the Makefile. `cmd/` holds only `main.go`: a plain switch that loads config and hands off to the package that does the work (`admin create` loads it itself, after its flags, so a usage error isn't hidden behind a config error). Arguments beyond a fixed word like `up` are parsed by that package, with the stdlib `flag` package where it needs flags; `family-hub -h` prints the usage. `serve` (default) hands off to `internal/app.Run`: migrate if `AUTO_MIGRATE=true` → pgxpool, which must answer a ping or startup fails → router → `http.Server` with read-header, read, write and idle timeouts → on SIGINT/SIGTERM, `Shutdown` lets in-flight requests finish for up to 8 seconds, under `docker stop`'s default 10-second grace before SIGKILL, then closes the rest, which cancels their queries. `main` turns SIGINT/SIGTERM into a canceled `ctx` for every command, then restores the default, so a second signal kills the process as usual. The port opens only after migrations and the pool are ready, so callers get "connection refused" while it starts rather than a hang. `serve` stopped by a signal exits 0, even during startup, unless shutdown overran its grace and had to close requests, or the signal came while `AUTO_MIGRATE` was running (a run whose last step finished counts as finished). Any other command that a signal cuts short exits 1, since its work may not have finished (at `admin create`'s password prompt the terminal is in raw mode, so Ctrl-C is read as a key that aborts). The server timeouts are constants at the top of `app.go`; the 2-second `/ready` timeout is at the top of `handler/health.go`. `/health` answers `ok` while the process runs; `/ready` answers `ok` only if the database answers a ping within 2 seconds, otherwise `503` (the error is logged, not returned). `migrate up` and `admin create` reuse the same config loader. If migration N fails halfway, golang-migrate records `version = N, dirty = true`. Either undo its partial changes and set `version = N-1, dirty = false` (the next `migrate up` retries N), or finish N by hand and set only `dirty = false`. Clearing `dirty` alone after a rollback would skip N for good. No `force` command needed. Sample data is `make seed`, which loads `internal/database/seed.sql` through the compose Postgres and refuses to run unless the database is empty (a check at the top of the file). The seeded test user stores a precomputed Argon2id hash of a known dev password.

### 6.5 Configuration (env vars)

`internal/config.Load` reads these and reports every invalid or missing value at once. Values are trimmed. `DATABASE_URL` must be a `postgres://` URL with a host or a `?host=` socket path; the prefix must be lowercase (pgx matches it case-sensitively), and the keyword/value form is rejected so the pool and migrations parse one format. It's checked with `net/url`, not pgx, whose parser also reads `PG*` variables and `~/.pgpass`; no error ever quotes it, since it holds the password. `AUTO_MIGRATE` is exactly `true` or `false`, and `HTTP_ADDR` needs a port from 1 to 65535. Each variable is added in the phase that uses it; Phase 1 has `DATABASE_URL` (required), `HTTP_ADDR` (default `:8080`) and `AUTO_MIGRATE` (default `false`). `.env.example` is the template for the server's `.env`, with safe local values only.

| Var | Example | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://app:app@localhost:5432/familyhub?sslmode=disable` | Required; URL form only |
| `HTTP_ADDR` | `:8080` | Default `:8080` |
| `BASE_URL` | `http://localhost:5173` (dev), `https://family.example.com` | Used in email links and the Origin check |
| `AUTO_MIGRATE` | `false` | Default `false`; `make dev-api` sets `true`. Booleans accept `true`/`false` in any case |
| `TRUSTED_PROXY` | `false` | Takes the client IP from the last `X-Forwarded-For` entry, the one the proxy appended |
| `COOKIE_SECURE` | `true` | `false` for local http |
| `SMTP_HOST/PORT/USER/PASS`, `MAIL_FROM` | `localhost:1025` (Mailpit, set by the Makefile), `Family Hub <noreply@family.example.com>` | Invites and password resets. Host and port are required by `serve` (no default, so a deploy without mail fails at startup) and ignored by `migrate up` and `admin create`. STARTTLS (port 587) is required for any non-loopback host; `SMTP_USER` and `SMTP_PASS` go together |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | from Google Cloud console | Optional: when unset, "Continue with Google" is hidden. Redirect URI is `<BASE_URL>/api/auth/google/callback` |
| `S3_ENDPOINT` | `http://storage:9000` | What the **server** uses |
| `S3_PUBLIC_ENDPOINT` | `http://localhost:9000` | What the **browser** uses; presigned URLs are signed against this host |
| `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_PATH_STYLE` | `auto` / `true` for R2 and local servers | |
| `MAX_UPLOAD_BYTES` | `2147483648` | Single-PUT cap (§9) |
| `MEMBERS_CAN_CONTRIBUTE` | `false` | Opens event creation, media upload and adding stories to members |
| `APP_TIMEZONE` | `America/New_York` | Defines "today" for the upcoming-birthdays panel and the home page's "Good evening" |
| `FAMILY_NAME` | `Torres` | Shown in the menu, page titles, emails ("the <FAMILY_NAME> Family Hub"); kept out of code |
| `FAMILY_SIDES` | `Torres,Ruiz` | Labels for the `first` and `second` family sides on the request form and the People filter |
| `BACKUP_PING_URL` | `https://hc-ping.com/<uuid>` | Pinged after each successful backup (§11) |

`S3_PUBLIC_ENDPOINT` matters because SigV4 includes the `Host` header: if the app runs in Docker and signs against `storage:9000`, the browser's request to `localhost:9000` fails signature validation.

## 7. Frontend

- **The design is set: `docs/design/`.** The mockup pages are the visual target and the build matches them as closely as the browser allows (layout, copy, colors, type, sizes, spacing, radii, motion); `docs/design/design-notes.md` is the behaviour spec and wins where the two disagree. Each page is compared side by side with its mockup before it's done, and a deliberate deviation is agreed with the owner first. The points below are the engineering side.
- **Look ("Heirloom"):** colors as CSS variables from day one (design-notes §1.1), so the deferred dark mode is a swap. Fraunces and Source Sans 3 (both SIL Open Font License) are **bundled as `woff2` files** and served by the app, not loaded from Google Fonts: the strict CSP (§12) and privacy both want same-origin fonts. Every animation is off under `prefers-reduced-motion`.
- React 19 + TypeScript + Vite; TanStack Query for server state; React Router.
- `vite.config.ts`: `server.proxy['/api'] = 'http://localhost:8080'`; `build.outDir = '../internal/webui/dist'`, `emptyOutDir: true`.
- Typed client: generate from an OpenAPI file later if the API grows; hand-written fetch wrapper for now with one error type.
- **Family tree view: a Workday-style focus view, not a canvas of the whole family.** The screen shows one **focus person** with their parents' household above, their own household beside them, and their children below: about two generations up and down. Siblings sit next to the focus person.
  - **Cards:** initials (photo once media lands in Phase 5), name, and the relation from the viewer's point of view. The tree shows no dates.
  - **Interaction (design-notes §4.2):** hover or keyboard focus shows a **quick look** under the card (relation, 2-line story from `bio_excerpt`, shared contact, "View profile", "Center here"); a click keeps it open, which is how phones get it; a double-click centers the tree on that person. ▲/▼ chips open more generations; a trail of recent focus people, "Back" and "Back to me" are always visible; a search box jumps to anyone. Pan and zoom aren't required.
  - **Quick-look contacts** load lazily: on hover or focus (debounced ~150 ms) the client calls `GET /api/persons/{id}` and caches it for the session; until it arrives, the quick look shows without the contact line.
  - **Households play the role of Workday's teams.** A couple, or a person with children and no partner on record, is one grouped card. Multiple marriages appear as separate households, with ended couples marked "former". Adoptive and foster links get a small label; step-relations are derived (§3.4) and labelled from the viewer's point of view. Households are derived from `relationships`, with no schema change.
  - **Layout:** only a few generations are drawn at a time, so plain CSS grid/flex is enough and no auto-layout engine is needed. There are no visible connector lines; the faint watermark tree behind the rows does that job (design-notes §4.2). Reach for elkjs or React Flow only if you later add a "whole family" overview.
  - **Data:** the client builds the view from the already-loaded `/api/tree` payload. A server-side "neighborhood" endpoint (§10) is a later optimization.
  - **Accessibility for older relatives:** large tap targets and type, high contrast, keyboard navigation, no hover-only actions, text labels instead of icon-only buttons.
  - **Prototype first:** build the focus view as a standalone clickable mockup at the start of Phase 4b, before the real tree, and show it to one non-technical relative. It uses no backend or database: the family data is hardcoded in the page, shaped like the `/api/tree` payload (§10) so the household-grouping logic can carry over into the real frontend. Real family shape and names are fine in the private mockup; anything committed to the repo uses made-up names. It is still the riskiest UI piece, but far smaller than a full-graph layout.
- Upload modal: request `upload-url`, `XMLHttpRequest`/`fetch` PUT with the returned headers (needed for progress events), then `confirm`. Compute width/height/duration client-side from the file for the confirm call.
- Auth pages: `/auth/login` (email + password, "Continue with Google", "Forgot password?", "Request access"), `/auth/request-access`, `/auth/forgot`, `/auth/set-password` (reads `#t=`; used for both invites and resets; for an invite it shows the Welcome page from `invite/preview`, then first-time setup), and "Change password" in My profile. Large inputs, a show-password toggle, and plain-language errors.
- **Member experience is read-mostly:** Home, the tree centered on "you", People, person profiles, a directory of visible contact info, Events, Stories, and a "My profile" page (bio, avatar, contact methods with visibility toggles, "Suggest a relative"). Tree-editing controls are not rendered for members.
- **Home ("Hearth"):** a greeting by time of day in `APP_TIMEZONE`, **Coming up** (upcoming birthdays §5.3, and upcoming events from Phase 5) and **New in the family** (people added in the last 30 days). Built from existing queries, no new tables.
- **Admin editing without raw edges:** admins never see "relationship type: parent_child, person A/B". From a person's card they choose "Add parent / child / partner / sibling", pick or create the other person, and the app maps that to a relationship (and, for "sibling", to shared parents). "Add parent" and "Add child" ask biological, adoptive or foster (default biological). "Add partner" asks **married or partner** and whether the relationship has **ended**; both can be changed later from the couple's card. Large icon buttons, plain-language labels, and undo (soft delete + restore) keep this safe for non-technical admins.

## 8. Tooling: Makefile, Compose, Dockerfile

### 8.1 Makefile (essentials)

The end state after Phase 1. Each target lands in the PR that needs it, so the real Makefile grows toward this one.

```make
DATABASE_URL ?= postgres://app:app@localhost:5432/familyhub?sslmode=disable
TEST_DATABASE_URL ?= postgres://app:app@localhost:5432/familyhub_test?sslmode=disable
TEST_MAILPIT_URL  ?= http://localhost:8025
SMTP_HOST         ?= localhost
SMTP_PORT         ?= 1025
export DATABASE_URL TEST_DATABASE_URL TEST_MAILPIT_URL SMTP_HOST SMTP_PORT

.PHONY: up down run dev-api dev-ui ui-stub generate test lint vuln build migrate-up migrate-new seed

up:            ; docker compose up -d --wait, then create familyhub_test if missing (see Makefile)
down:          ; docker compose down

# Creates a placeholder so `go build/vet/test` work before the UI has been built.
ui-stub:
	@test -f internal/webui/dist/index.html || \
	  (mkdir -p internal/webui/dist && printf '%s\n' '<!doctype html><title>Family Hub</title><p>The UI is not built yet.</p>' > internal/webui/dist/index.html)

# sqlc output + mockery mocks; both committed, CI fails if they're stale
generate:      ; go tool sqlc generate && mockery   # after checking the mockery version, as lint does
migrate-new:   ; go tool migrate create -ext sql -dir internal/database/migrations -seq $(name)
migrate-up: ui-stub ; go run ./cmd migrate up
seed: up migrate-up ; docker compose exec -T postgres sh -c 'psql -q -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < internal/database/seed.sql

dev-api: up ui-stub ; AUTO_MIGRATE=true COOKIE_SECURE=false go run ./cmd
dev-ui:        ; cd web && npm run dev
run: ui-stub   ; COOKIE_SECURE=false go run ./cmd $(ARGS)

test: ui-stub ; env -u DATABASE_URL go test -race ./...   # tests can't reach the dev database
lint: ui-stub ; golangci-lint run && cd web && npm run lint && npm run typecheck
vuln: ui-stub  ; go tool govulncheck ./...

build: ui-stub
	cd web && npm ci && npm run build
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/family-hub ./cmd
```

`sqlc`, `govulncheck` and the `migrate` CLI are pinned in `go.mod` via `go get -tool ...` (Go ≥1.24), so there's no `@latest`. mockery (v3) and golangci-lint are installed as pinned binaries instead, locally and in CI, because both projects advise against installing via `go get`/`go tool` (dependency conflicts; mockery's docs recommend the release binary, checked 2026-11). The Makefile's `lint` and `generate` refuse any other version; CI downloads mockery's release tarball and checks a pinned SHA-256. `go tool migrate` is only used for `create`; `migrate up` goes through the app binary, so the CLI doesn't need database build tags **(verify)**.

### 8.2 `compose.yaml` (shape)

Local development only. Ports bind to `127.0.0.1`. Images are pinned to exact version tags; Dependabot groups their updates into one weekly PR and skips Postgres major versions (those need a dump and restore). `make up` creates `familyhub_test` if it's missing, so it works on an existing volume too. CI won't run `make up`: the `go` job's Postgres service container sets `POSTGRES_DB: familyhub_test` directly.

```yaml
services:
  postgres:
    image: postgres:18.6
    environment: { POSTGRES_USER: app, POSTGRES_PASSWORD: app, POSTGRES_DB: familyhub }
    ports: ["127.0.0.1:5432:5432"]
    volumes: [pgdata:/var/lib/postgresql]   # Postgres 18 images keep data under /var/lib/postgresql/18/docker
    healthcheck:                            # over TCP, so first-run setup doesn't count as ready
      { test: ["CMD-SHELL", "pg_isready -h 127.0.0.1 -U \"$$POSTGRES_USER\" -d \"$$POSTGRES_DB\""], start_period: 30s, interval: 2s, retries: 15 }
  mailpit:                       # catches outgoing mail; UI on :8025
    image: axllent/mailpit:v1.33.1
    ports: ["127.0.0.1:1025:1025", "127.0.0.1:8025:8025"]
  storage:                       # added in Phase 5: S3-compatible server chosen then (see §9.5)
    # image: ...
volumes: { pgdata: {} }
```

### 8.3 Dockerfile (shape)

```dockerfile
# Base images are pinned to exact tags; Dependabot proposes updates.
# The Go version must be at least the `go` line in go.mod.
FROM golang:1.27.3-alpine3.24 AS build
WORKDIR /src
RUN apk add --no-cache make
COPY . .
# The Makefile's build, so the image and CI build the same way (with the placeholder UI until Phase 4a).
# No `go mod download`: it would also fetch the sqlc tool's large dependency tree.
RUN --mount=type=cache,target=/go/pkg/mod make build

# Phase 6 adds ffmpeg and vips here.
FROM alpine:3.24.4
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=build /src/bin/family-hub .
EXPOSE 8080
# The port from HTTP_ADDR, trimmed and defaulted as config.Load does; the app must listen on all interfaces or loopback.
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s \
  CMD addr=$(echo $HTTP_ADDR); addr="${addr:-:8080}"; wget -qO- "http://127.0.0.1:${addr##*:}/health" || exit 1
ENTRYPOINT ["./family-hub"]
```

Phase 4a puts a Node stage in front, and the build stage then compiles only the Go part (`make build` will also run npm, which this stage doesn't have):

```dockerfile
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
# vite outDir is ../internal/webui/dist
RUN npm run build
# ...and in the build stage, after COPY . .:
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/family-hub ./cmd
```

The binary is PID 1 (exec-form `ENTRYPOINT`), so `docker stop`'s SIGTERM reaches it directly and the 8-second shutdown (§6.4) fits in Docker's 10-second grace.

`.dockerignore` (keeps `node_modules` and local build output out of the build context, so `COPY . .` doesn't drag in gigabytes):

```
.git
.github
.idea
.vscode
.DS_Store
bin
docs
**/node_modules
web/dist
internal/webui/dist
*.md
.env*
```

The image builds its own `dist/` (the placeholder until Phase 4a, then the web stage), so excluding the local one is correct. `.env*` keeps local secrets out of the build context.

## 9. Media pipeline

### 9.1 Upload

1. `POST /api/media/upload-url {filename, mime_type, size_bytes, event_id?}`. Server validates the MIME against an allowlist (`image/jpeg`, `image/png`, `image/webp`, `image/heic`, `image/tiff` for flatbed scans of old photos, `video/mp4`, `video/quicktime`, and for stories `audio/mpeg`, `audio/mp4`, `audio/wav` (old cassette transfers); TIFF and HEIC are shown through the Phase 6 JPEG variant, since browsers can't display TIFF) and `size_bytes <= MAX_UPLOAD_BYTES`; inserts a `pending` row with key `media/<yyyy>/<uuid>/original.<ext>` (extension derived from MIME, **never** from the filename); returns a presigned PUT (15 min) that signs `Content-Type` and `Content-Length`.
2. Browser PUTs directly to the bucket. The response's `headers` contain only `Content-Type`; browsers forbid scripts from setting `Content-Length` and send it automatically from the file body, which must equal the signed value. A SigV4-signed `Content-Length` makes the storage service reject mismatches, which is what enforces the size cap. **(verify on the chosen storage server and on R2 in the Phase 5 storage spike)**; the `HeadObject` size check in step 3 is the backstop either way.
3. `POST /api/media/{id}/confirm` → server calls `HeadObject`, checks it exists and `ContentLength == size_bytes`, sets `status='ready'`, records tags/caption, and (Phase 6) sets `processing_status='queued'`.

Single PUT is capped at 5 GiB on S3/R2; I'd set a lower app cap (e.g. 2 GiB) and add multipart uploads only if you hit it.

### 9.2 Viewing

`GET /api/media/{id}/url?variant=` returns a presigned GET (1 h) for the chosen variant. `GET /api/media` embeds thumbnail URLs so a grid is one request. Video seeking works because the bucket honors `Range`. Presigned URLs aren't browser-cacheable across expiry, which is acceptable for a family-sized library. Until the Phase 6 worker exists there are no variants: photo grids use the original as the thumbnail and videos show a placeholder tile.

### 9.3 Orphans and deletion

- Hourly sweep: `pending` rows older than 24 h → delete object if present, mark `failed`.
- Soft delete sets `deleted_at`; a daily job deletes bucket objects only after a 30-day grace period. Bucket lifecycle rule aborts incomplete multipart uploads.
- Bucket CORS: allow `PUT`/`GET`/`HEAD` from `BASE_URL`, expose `ETag`.

### 9.4 Processing worker (Phase 6)

In-process goroutine(s) polling `SELECT ... WHERE processing_status='queued' FOR UPDATE SKIP LOCKED`.

- **Video:** ffmpeg → poster JPEG, H.264/AAC MP4 (720p, `+faststart`) as `mp4_720`. This fixes iPhone HEVC `.mov` playback in Chrome/Firefox. Extract width/height/duration with `ffprobe`.
- **Photo:** thumbnail + display-size JPEG; HEIC → JPEG via `vips`/`vipsthumbnail` (or ffmpeg) **(verify HEIC decoding is present in the chosen Alpine packages, e.g. `vips-heif`)**. Read EXIF date into `taken_on` (precision `day`) when the admin hasn't set one, and apply rotation.
- Output keys go in `media.variants`; failures set `processing_status='failed'` and the UI falls back to "download original".
- CPU-heavy transcodes on a small VPS will be slow; the queue keeps the API responsive. If it becomes a problem, move the worker into a second container from the same image (`family-hub worker`).

### 9.5 Local object storage: decided at the start of Phase 5

Nothing before Phase 5 stores files, so until then the local stack is just Postgres and Mailpit, and no Cloudflare/R2 account is needed.


MinIO was the V2 default. Its community edition is in maintenance mode and has been source-only (no new Docker images) since about October 2025. Candidates: a **pinned last MinIO image** (current lean: dev-only, so being frozen matters little), **Garage** (fallback; maintained, lightweight), RustFS (newer, less proven), SeaweedFS (heavier than needed), or no local server and a dev R2 bucket (loses offline dev). The spike runs the same checks against real R2 too, since R2's behavior is what matters in production. Whatever is chosen sits behind the `storage.Storage` interface, so the choice is cheap to reverse. The spike must prove: presigned PUT with signed `Content-Length`, `HeadObject`, presigned GET with `Range`, and browser CORS.

### 9.6 Stories (Phase 5b)

A story is a recorded video or audio of a relative telling something, with chapters and a transcript (design-notes §4.6–4.7b). Its file is an ordinary `media` row, uploaded the normal way (§9.1).

- **Transcripts are typed or pasted by an editor** in the MVP, as timestamped lines; chapters likewise. Captions (`captions.vtt`, on by default in the player) are generated from the transcript. Machine transcription (self-hosted Whisper or a hosted speech-to-text API) is a later, separate decision: it's a new dependency, and Haitian Creole accuracy needs a human read-through anyway.
- **Consent:** a story is published only once `consent_confirmed_at` is set; drafts are visible to their creator and admins. Its recording follows the story: media referenced by a story are left out of `GET /api/media` (Stories lists them, not Media), and `GET /api/media/{id}` and `/url` for them answer only to those who may see the story, so a draft's recording can't be reached before consent.
- **Search** covers what was said: the service keeps `search_tsv` (title, summary, transcript) current on every write; `q` uses it. People mentioned who aren't in the tree stay text in the transcript or summary; only tree people go in `story_persons`.
- **Profiles** list "Stories <name> told" (`teller_person_id`) and "Stories about <name>" (`story_persons`). A story's year can appear on the Events timeline.
- **Length:** long recordings can pass the 2 GiB single-PUT cap (§9.1). Until multipart uploads exist, the Add a story page says so and suggests a smaller export.
- **Playback:** until the Phase 6 worker, stories must be H.264 MP4 or audio; an iPhone HEVC `.mov` won't play in Chrome or Firefox.

## 10. Family-tree queries

- **`GET /api/tree`** is two flat reads: all non-deleted `persons` (slim columns) and all `relationships` whose endpoints are both non-deleted. At family scale (hundreds to low thousands of people) this is a few hundred KB; no recursion needed. The client builds union nodes and runs the layout. The payload carries **no birth or death dates** (the tree never shows them; `is_deceased` stays for labels), which also removes one place the §5.2 masking could leak, and **no contacts**. Each person has a `bio_excerpt` (the first ~140 characters of `bio`) for the quick look; contacts come from `GET /api/persons/{id}` (§7).
- **Optional later:** `GET /api/persons/{id}/neighborhood?up=2&down=2` returns just the households around one person (same masking), if the full payload ever gets too large for phones. Not needed at family scale.
- **One masked person read.** Every query that returns people to a browser selects explicit columns, never `p.*` or `birth_date`, so neither dates nor anything added to `persons` later can leak through `/api/tree`. Those that return birth dates (`/api/persons`, `/api/persons/{id}`, ancestors, descendants, directory, event and media tags) returns `masked_birth_date(p, $viewer_is_admin, $viewer_person_id)` (defined in Phase 3's migration with `persons`, §3.1) and never selects `birth_date` or `p.*`. sqlc can't share column lists between queries, so the SQL function is the single place the rule lives, and a living person's birth year can't leak through an endpoint added later.
- **Ancestors / descendants** use recursive CTEs with a depth bound, over **family-parent links only** (`biological`, `adoptive`; §3.4), so foster links never appear as ancestors or descendants:

```sql
-- name: Ancestors :many
WITH RECURSIVE anc AS (
  SELECT r.person_a AS id, 1 AS depth
  FROM relationships r
  WHERE r.type = 'parent_child' AND r.subtype IN ('biological','adoptive')
    AND r.person_b = sqlc.arg(person_id)
  UNION ALL
  SELECT r.person_a, a.depth + 1
  FROM relationships r
  JOIN anc a ON r.person_b = a.id
  WHERE r.type = 'parent_child' AND r.subtype IN ('biological','adoptive')
    AND a.depth < sqlc.arg(max_depth)
)
-- never p.* : birth dates go through masked_birth_date (see above)
SELECT p.id, p.first_name, p.last_name, p.is_deceased,
       masked_birth_date(p, sqlc.arg(viewer_is_admin), sqlc.arg(viewer_person_id)) AS birth_date,
       p.birth_date_precision, min(anc.depth) AS depth
FROM anc JOIN persons p ON p.id = anc.id AND p.deleted_at IS NULL
GROUP BY p.id;
```

- **Cycle prevention at write time.** Inside one transaction that first takes `pg_advisory_xact_lock(<constant>)` (relationship writes are rare, so serializing them is free, and it stops two concurrent inserts from jointly forming a loop), reject "A is parent of B" if B is already an ancestor of A. Unlike the display queries, this walk has **no depth bound**, follows **every** parent link (foster included) and ignores `deleted_at`, so it is a guarantee rather than a heuristic. It uses `UNION`, not `UNION ALL`: already-visited people are dropped, so the recursion always terminates, even on damaged data.

```sql
-- name: IsAncestor :one
WITH RECURSIVE anc(id) AS (
  SELECT person_a FROM relationships
  WHERE type = 'parent_child' AND person_b = sqlc.arg(parent_id)
  UNION
  SELECT r.person_a FROM relationships r
  JOIN anc ON r.person_b = anc.id
  WHERE r.type = 'parent_child'
)
SELECT EXISTS (SELECT 1 FROM anc WHERE id = sqlc.arg(child_id));
```

  Couple edges aren't traversed, so they can't create loops. The same locked transaction also rejects a third **biological** parent for the child (§3.3).
- **Name search** queries the `search_name` generated column (§3.1), which the trigram index covers, so query and index can't drift apart. To make middle or maiden names searchable later, change the column's expression in a migration.
- Direction convention is enforced by the service, not just documented: the API accepts `{type:"parent_child", parent_id, child_id}` and maps to `person_a/person_b`; for couples it sorts the pair so `person_a < person_b`.

## 11. Backups and durability

For a "multi-decade" archive, this section matters more than any other.

- **Postgres:** nightly `pg_dump -Fc` (run from a `postgres:18` container so the client matches the server) to a bucket in a **different account/provider** than the media bucket, 30 daily + 12 monthly retention. Managed Postgres with PITR is a bonus, not a substitute.
- **Media:** originals are never deleted synchronously (§9.3). Periodically `rclone sync` originals to a second location (external disk, Backblaze B2, or S3 Glacier). R2 doesn't offer object versioning as far as I know **(verify)**, so this copy is your protection against accidental deletes and account problems.
- **Know when a backup fails:** the backup job pings `BACKUP_PING_URL` (a free dead-man's-switch service such as healthchecks.io) only on success. If no ping arrives in ~26 h, you get an email. Without this, a broken backup is discovered only when you need it.
- **Succession:** name a second admin and keep a one-page handover note (where the VPS, domain, R2, mail provider and backup buckets live; how to renew the domain; how to run a restore), stored somewhere they can reach without you. A multi-decade archive shouldn't depend on one person's accounts.
- **Restore drill:** a documented, scripted restore (`make restore-drill`) into a scratch DB + bucket, run once in Phase 4c, again in Phase 7 once media exists, and then twice a year. An untested backup is a hope.
- **Export:** a `family-hub export` command later (people/relationships/events as JSON + GEDCOM) keeps the data portable beyond this app.

## 12. Security & privacy checklist

- Invite-only; enumeration-safe login, forgot-password, access-request and Google matching; the public request form never suggests or reveals names from the tree; Argon2id password hashes; single-use 256-bit emailed tokens; Google sign-in only links to an already-invited, verified email; server-side revocable sessions, revoked on password reset (§5).
- Security headers: strict CSP (`default-src 'self'; frame-ancestors 'none'`; `img-src`/`media-src` allow the bucket host), `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, `X-Robots-Tag: noindex`, HSTS in prod.
- All SQL via sqlc (parameterized). JSON body size limits. Upload MIME allowlist; keys never derived from user filenames.
- Secrets only from env/secret manager; separate S3 credentials for app vs. backup jobs; the app's key can't delete from the backup bucket.
- Contact info is private by default, filtered server-side per viewer (§5.2), excluded from the tree payload, search and logs, and never includes the login email.
- Family photos include children and living people: HTTPS only, no public buckets, no search-engine indexing, short-lived presigned URLs.
- Structured logs (zerolog JSON on stderr) without tokens, cookies or presigned URLs. Each request logs one line: method, path without its query (it can carry search terms), status, bytes, duration and a server-made request ID (also sent as `X-Request-Id`). Successful `/health` and `/ready` probes are left out.

## 13. Testing & CI

- **Go:** unit tests for services; integration tests against real Postgres (`familyhub_test` DB from compose via `TEST_DATABASE_URL`, never `DATABASE_URL`; `internal/testdb` holds the skip-locally, fail-in-CI rule, schema built by running the embedded migrations; truncate between tests). **Mocks:** mockery generates testify mocks (`.mockery.yml`, committed) for the interfaces at the edges and between layers: `Mailer` and `Storage` (into `internal/mail/mocks/`, `internal/storage/mocks/`), and the service interfaces declared in `handler` (into `internal/handler/mocks/`). The database is **never** mocked; query and permission behavior is tested against real Postgres. Tests are **table-driven** by default (a slice of named cases looped with `t.Run` subtests). Assertions use testify (`require` for setup, `assert` for checks). Time-dependent code (session expiry, rate limits, upcoming-birthday window) is tested with `testing/synctest` instead of sleeps. Specific tests: ancestry-cycle rejection (including a chain deeper than any display bound, and a loop closed through a foster link), ancestors/descendants exclude foster links, relationship canonicalization/uniqueness (one couple row per pair), third biological parent rejected (via POST and via PATCH changing a subtype), missing `subtype` defaults or returns 400, `ended` only on couples, death date requires `is_deceased`, search works for people without a last name, blank or whitespace-only names are rejected (a blank surname input is stored as NULL), derived relations (full/half/unspecified siblings, including one sibling's second parent not yet recorded and people with more than two family parents, step-parents incl. former, step-siblings; adoptive links behave like biological; foster links yield only foster labels, e.g. two foster children of one couple are foster siblings, not full siblings), emailed tokens single-use and expiring (invite 7 days, reset 30 min), reset revokes all sessions, login returns the same 401 for unknown email / wrong password / Google-only / disabled user, every login failure path runs one Argon2 verify (unknown, Google-only and disabled users alike), any credential change invalidates all outstanding tokens, disabled users are refused on every path including Google by `sub`, admin email change clears `google_sub` and the password and revokes sessions, a new reset request leaves earlier reset links valid, token consumption is all-or-nothing (a failure mid-way leaves the token unconsumed and the old password intact), concurrent credential changes for one user serialize on the row lock, emails are matched case-insensitively (incl. Google's), per-account failure limits and the 50/hour backstop never block Google or reset, `password/change` is rate-limited, the hash semaphore caps concurrent Argon2 work, Argon2id rehash when parameters change, password length and common-password rules, Google sign-in: repeat sign-in with the user's existing `sub` succeeds; an email auto-link attempt with a **different** `sub` for a user already linked is rejected, as are unverified emails, uninvited emails and state/nonce mismatches, and auto-links only Gmail or matching-`hd` Workspace addresses (a verified non-Gmail address without `hd` is refused; that user signs in with their password), and links `google_sub` on first use, a login whose verified hash changed before the session commit is refused (no stale session, no rehash over a new password), Google sign-in by `sub` is refused if the email was changed or the user disabled mid-flow (and a flow started before an email change can't re-link by email), the auth cleanup job deletes expired/consumed flows, tokens and sessions, a forged, expired or reused flow handle is refused, change password always requires the current password and is limited per user, session sliding/absolute expiry, rate limits, `confirm` rejecting wrong size, an access request answers the same for a member's email, a pending request's email and a new one, and never creates a user; approving a request through an invite marks it approved in the same transaction; a declined request sends no email; `invite/preview` consumes nothing and refuses used or expired tokens like `password/reset`; `/api/tree` carries no dates and no contacts; a story can't be published without consent, and drafts are hidden from other members; story search finds a word from the transcript. Permission tests are table-driven across roles: a member can't create/patch other people or touch relationships; a member can edit only their own contacts; hidden contact rows never appear in `/api/persons/{id}`, `/api/directory`, `/api/tree` or search for non-owners; the unique person-link constraint; refusing to delete a linked person; birth-year masking (living vs deceased, self, admin, across every endpoint that returns birth dates: search, profile, ancestors, descendants, directory, upcoming birthdays; `/api/tree` returns none); upcoming birthdays: Feb 29 handling, year-end wraparound (Dec 28 → Jan 5), deceased/deleted people skipped.
- **Web:** Vitest + Testing Library for components; one Playwright smoke test (seed a user with a known password → sign in → add person → see tree) once Phase 4b lands.
### 13.1 Branches, CI and releases

**Branches (trunk-based):** `main` plus short-lived work branches named `<type>/kdelpe/<descriptive-name>` (types `feature`, `fix`, `docs`, `chore`; e.g. `feature/kdelpe/password-login-and-reset`). Every change reaches `main` through a PR that is **squash-merged**, so `main` has one commit per PR. Releases are tags on `main`.

**`ci.yml`** runs on PRs into `main` and on pushes to `main`. Each job lands once there's something for it to check (`generated` with sqlc, `web` with the frontend, `docker` with the Dockerfile):

| Job | Checks |
|---|---|
| go | `go mod tidy -diff`, golangci-lint (its `standard` linters include `go vet`, plus the gofmt formatter; `.golangci.yml`), `govulncheck`, `go test -race` (via `make test`) with a `postgres:18.6` service whose `POSTGRES_DB` is `familyhub_test`, `make build` |
| generated | Deletes `internal/database/gen` and every `mocks/` folder, then runs `make generate` (sqlc and mockery); fails if the working tree then differs, so stale or orphaned output is caught |
| web | Lint, typecheck, tests, build |
| docker | `docker build` (no push) |

**Releases** are cut by the owner in the GitHub UI. **`draft-release.yml`** runs on every push to `main` (each squash-merge) and keeps exactly one draft Release. With no draft, it creates one (`gh release create --draft`) suggesting the next patch after the highest published `vX.Y.Z` as the tag (`v0.1.0` when there is none). Releases are never deleted and prereleases aren't used, so that tag is always free. With a draft, it updates it in place: new target commit and regenerated notes since the last published release (notes for both paths come from the `releases/generate-notes` API, which accepts a tag that doesn't exist yet when given a target commit; the previous tag is passed explicitly because the API doesn't document a default), but the draft's tag name is kept, so a tag the owner changed survives, and it re-checks that the release is still a draft just before editing, so one published meanwhile keeps the owner's notes. It never deletes the draft, so a draft open in the UI can still be published. A `concurrency` group runs one at a time, so back-to-back merges can't create two drafts (a newer queued run replaces an older one, which is fine). The owner changes the tag for a minor or major bump, edits the notes just before publishing (a later merge regenerates them), and clicks Publish, which creates the tag. **`release.yml`** runs on `release: released` (a full release is published, or a prerelease is promoted to one), so a `-rc` prerelease never produces a deployable image: it checks out the tag with full history, fails unless the tag is `vX.Y.Z` and the tagged commit is on `main` (`git merge-base --is-ancestor`), then builds the image and pushes it to GitHub Container Registry as `ghcr.io/<owner>/<repo>:<tag>` with plain `docker login`/`build`/`push` and the workflow's `GITHUB_TOKEN` (no third-party actions). It refuses to push a tag that already has an image (a lookup that fails for any reason but "manifest unknown" also stops it), and runs one at a time per tag, so a version's image never changes. A package pushed by a workflow is linked to its repository and starts **private**; the VPS pulls it with a read-only token. The image tag is the version; the binary doesn't embed one. A tag ruleset on `v*` blocks updating or deleting release tags. Deploying is a separate, deliberate step (the VPS pulls the tagged image).

**Workflow hygiene:** least-privilege `permissions:` per workflow (read-only by default; `draft-release.yml` gets `contents: write`, the release job gets `contents: read` and `packages: write` (listing any permission sets the rest to none)); third-party actions pinned to commit SHAs; Dependabot keeps them current.

**Repository settings (not in files):** `main` requires a PR (0 approvals, since the owner reviews their own PRs), the `go`, `generated` and `docker` CI checks must pass on a branch that is up to date with `main` (add each new CI job here as it lands), no direct pushes or force-pushes (enforced for admins too), no deletion. Squash merge is the only merge method enabled.

## 14. Deployment (Phase 4c options)

| Option | Shape | Pros | Cons |
|---|---|---|---|
| **A. Single VPS (recommended start)** | Docker Compose: app + Postgres (volume) + Caddy (automatic TLS); media on R2 | Cheapest, simplest, no cold starts; you own backups | You patch the box |
| B. Managed | App on Fly.io/Cloud Run + Neon/Cloud SQL/RDS (must offer Postgres 18) + R2 | Less ops | Cost, cold starts (Neon scale-to-zero), more moving parts |

**Health checks stay internal:** Caddy answers `404` for `/health` and `/ready` from the internet; Docker's `HEALTHCHECK` and any monitoring call them on the server itself (`127.0.0.1:8080`). The app also never serves them under `/api`.

Either way: `TRUSTED_PROXY=true` only when behind Caddy/the platform proxy, `COOKIE_SECURE=true`, `BASE_URL` set to the real domain, R2 CORS configured for it, and the §11 jobs scheduled before real family data is entered.

## 15. Phased roadmap

**MVP = Phases 1 through 5b**: a working tree, logins, access requests, contact visibility and upcoming birthdays, events with photos and video, and family stories, **deployed with backups and in the family's hands**. The transcoding worker (Phase 6) comes only once you've actually hit a playback problem.

Real family data gets entered from Phase 4 onward, so Phase 4c deploys with backups before any media work starts, and the family can start using it then. Nothing precious should live only on a laptop. Phases 5 and 5b complete the MVP on that deployment.

| Phase | Goal | Exit criteria |
|---|---|---|
| **1. Foundation** | Repo, compose, config, migrations, health, CI, `webui` embed + SPA handler, draft-release and release workflows | `make dev-api` runs (it starts compose itself); `/health` + `/ready` pass; CI green on a fresh clone (stub UI) |
| **2. Auth** | Invite-only email + password (Argon2id), invite and forgot-password emails, invite preview, change password, sessions, rate limits, `admin create`, admin user endpoints. Then Continue with Google as a separate PR | Invite → set password via Mailpit → sign in; forgot/reset works and revokes sessions; Google sign-in links an invited account and refuses an uninvited one; enumeration, replay, expiry and disabled-user tests pass |
| **3. People, tree & contacts API** | Person linking (§5.1, moved from Phase 2 so it lands with the `persons` table), invites "from" a person, access requests and member suggestions with tree matching, admin-only persons/relationships writes, `/api/tree`, ancestors/descendants, cycle validation, contact methods with visibility filtering, `/api/directory` | Integration tests incl. cycle rejection and the role/visibility matrix (§13); one-login-per-person enforced; seed script loads a sample family |
| **4a. Frontend shell** | The design's shell and pages (`docs/design/`): sign in, request access, Welcome + first-time setup, forgot password, Home, People, person profile, directory, "My profile" (bio and contact visibility toggles; initials only, avatars arrive in Phase 5), Members & invites with the Requests queue | Accept an invite via Mailpit, set a password, sign in and land on the home page; forgot-password works end to end; edit own bio and contact visibility; a member sees only visible contacts in profiles and the directory; works on desktop and phone |
| **4b. Family tree** | **Mockup first:** a standalone clickable focus-view mockup (§7) with the real family shape, tried on the owner's phone, then by one non-technical relative. Then the real focus-view family tree centered on "you" (§7), "back to me", search-to-jump, admin add-parent/child/partner/sibling flows (§7) | Sample family can be navigated by clicking, with multiple marriages and step-children shown as households, on desktop and phone; an admin can build a branch without touching raw edges |
| **4c. First deploy** | Deploy (§14), mail provider + domain with SPF/DKIM/DMARC, nightly backups + failure alert, restore drill, TLS, basic security headers | Deployed behind TLS; login email lands in Gmail and Outlook inboxes; restore drill passes; a deliberately failed backup triggers the alert; first real invites sent |
| **5. Events & media (originals)** | **Storage spike first:** choose the local server and prove the four capabilities in §9.5 against it and R2 with a throwaway Go test; add `storage` to compose. Then events + feed, upload/confirm/view (admin-only at first; members via `MEMBERS_CAN_CONTRIBUTE`), avatars (members can always set their own), tagging, orphan sweep, admin "missing info" tracker (§5.4) | Upload a 1 GB video from the browser and play it back with seeking |
| **5b. Stories** | Stories with chapters, typed transcripts, captions, search, consent; Stories, A story and Add a story pages (§9.6) | Add a story with an audio file and a video file, publish it after confirming consent, find it by a word from its transcript, and play it with captions on desktop and phone |
| **6. Processing** | Worker: thumbnails, posters, H.264 variant, HEIC, EXIF | An iPhone HEVC `.mov` plays in Chrome and Firefox |
| **7. Harden** | Strict CSP covering the media bucket, runbook + handover note, export command if wanted (§11) | Second admin named and handover note written; CSP has no violations in normal use; restore drill re-run with media in the backups |

## 16. Decisions

**Decided.** These defaults favor the simplest option; each is easy to revisit.

1. **Roles:** admins maintain the tree, events and media; members edit only their own profile and contact info. One flag, `MEMBERS_CAN_CONTRIBUTE` (default `false`), later opens event creation and media upload to members.
2. **Visibility:** two levels for contact info (`hidden` / `family`). Birthday month and day are visible to all; the birth year of living people is visible to admins and that person only. Admins see everything.
3. **No birthday emails.** Birthdays appear only in the app (§5.3). The app emails nothing but invites and password resets.
4. **Access:** invite-only, but relatives can **request access** and members can **suggest a relative** (reversed in November 2026 by the frontend design, §5). Sending invites stays admin-only. Still no "suggest a correction" feature; relatives who spot a mistake message the admin.
5. **Video:** one 720p H.264 variant, built only when you hit a playback problem (Phase 6).
6. **Hosting:** Option A, a single VPS (§14).
7. **GEDCOM import/export:** later. Move it up only if you already have a tree file from another service.
8. **Stories are in the MVP** (Phase 5b), with transcripts typed by hand; machine transcription is a later decision (§9.6).
9. **Frontend design:** `docs/design/` is the agreed look and behaviour, and the build matches the mockups (§7).

**Deferred from the frontend design** (accepted as ideas, not scheduled):

- Social accounts as contact methods (WhatsApp, Instagram, …), with links built from each platform's known base URL.
- Phone numbers and addresses shared **on request**: a member asks and the owner shares with that one person (a `contact_requests` table).
- "On this day" on Home, and a stored theme preference for dark mode (`users.theme`).
- Machine transcription, recording stories in the browser, and multipart uploads for long recordings.
- "Suggest a correction" on profiles, feeding the same Requests queue.

**Still open, but not blocking until later**

- **Local storage server:** settled by the storage spike at the start of Phase 5 (§9.5).
- **Mail provider and sending domain:** needed before the first real invites, because invite and password-reset emails depend on it (locally, Mailpit covers everything). My recommendation: register a cheap dedicated domain and use it for both the site and email, sending through a transactional provider with a free tier. Current lean: **Resend** (3,000/month free, SMTP), with Postmark as the switch-by-config alternative; register the domain at Cloudflare Registrar (DNS alongside R2), auto-renew on, several years prepaid, and listed in the handover note. **Verify current limits at Phase 4c.** Add the SPF/DKIM/DMARC records the provider gives you, then test delivery to Gmail and Outlook.
