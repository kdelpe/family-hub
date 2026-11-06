# Family Hub: Architecture & Engineering Plan V3

A private, invite-only, multi-generational family archive: interactive family tree, event/milestone feed, photo and video library, passwordless sign-in. One Go service serves the JSON API and the embedded React SPA. Postgres holds data; an S3-compatible bucket holds media, uploaded directly from the browser.

**Status:** draft for review. Items marked **(verify)** are things I believe but haven't confirmed against current docs; check each one when the phase that depends on it starts.

---

## 0. What changed from V2

| # | V2 problem | V3 fix |
|---|---|---|
| 1 | `ui.go` was `package main` at repo root; `cmd/server` can't import it and `go:embed` can't reach `..` | Embed lives in `internal/webui` (package `webui`); Vite builds into it (§2, §7) |
| 2 | Fresh clone fails `go build`/`go test` because `dist/` doesn't exist | `make ui-stub` writes a placeholder; `dist/` is git-ignored (§8.1) |
| 3 | Makefile `POSTGRES_URL` garbled in the PDF | Rewritten; migrations also run via the binary itself (§8) |
| 4 | 6-digit code: brute-forceable, replayable, bare SHA-256, no session storage | 256-bit single-use link token, server-side sessions, rate limits, enumeration-safe (§5) |
| 5 | `RealIP` trusted unconditionally | Only enabled behind a configured trusted proxy (§6) |
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
| Language | Go (≥1.25 is plenty; confirm the 1.27 toolchain with `go version`) | Nothing here needs 1.27 specifically. `go tool` directives need ≥1.24. |
| HTTP | chi v5 | stdlib `net/http` compatible |
| DB access | pgx v5 pool + **sqlc** | Typed queries from `.sql` files; no runtime reflection |
| Migrations | golang-migrate, embedded, pgx v5 driver | Auto-run gated by `AUTO_MIGRATE` |
| Storage | S3 API via aws-sdk-go-v2 | Local server chosen at the start of Phase 5 (§9.5); R2 in prod |
| Mail | SMTP (`net/smtp`) behind a `Mailer` interface | Mailpit local; Resend/SES/Postmark SMTP later |
| Frontend | React 19 + TypeScript + Vite, TanStack Query | Focus-view family tree (§7); plain CSS + SVG connectors |
| Jobs | Postgres queue (`FOR UPDATE SKIP LOCKED`) in-process | No Redis/SQS needed at this scale; added in Phase 6 |
| Stdlib where nothing else is needed | `log/slog` (logs), `flag` (subcommands), `os.Getenv` + validation (config), `http.CrossOriginProtection` (CSRF, Go 1.25), `crypto/*` | No cobra, no config library |
| Testing | `testing` + **testify** (assertions, mocks) + **mockery** (generated mocks) + `testing/synctest` (fake clock, Go 1.25) | Database is never mocked: real Postgres in integration tests (§13) |
| Lint & security | golangci-lint, `govulncheck` | |
| CI / releases | GitHub Actions | `.github/` (§13.1) |

```
family-hub/
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                # checks on PRs into main and pushes to main (§13.1)
│   │   └── release.yml           # manual release, main only (§13.1)
│   ├── pull_request_template.md
│   ├── CODEOWNERS
│   └── dependabot.yml            # Go, npm, Actions, Docker update PRs -> main
├── cmd/family-hub/main.go        # subcommands (flag): serve (default) | migrate | admin | seed
├── internal/
│   ├── app/                      # builds dependencies (pool, services, handlers), server lifecycle, graceful shutdown
│   ├── router/                   # chi route table + middleware chain; the only place routes are wired (§6.1)
│   ├── handler/                  # all HTTP handlers, one file per area: health.go auth.go admin.go
│   │                             #   family.go contacts.go event.go media.go; service interfaces they use
│   │   └── mocks/                # mockery mocks of those service interfaces
│   ├── config/                   # env loader + validation
│   ├── httpx/                    # JSON helpers, error type, pagination, security middleware
│   ├── auth/                     # magic links, sessions, rate limits                (Phase 2)
│   ├── admin/                    # invites, roles, person linking                    (Phase 2)
│   ├── family/                   # persons, relationships, tree, ancestors, cycles   (Phase 3)
│   ├── contacts/                 # contact methods, visibility, directory            (Phase 3)
│   ├── event/                    #                                                   (Phase 5)
│   ├── media/                    # upload/confirm/view; worker in Phase 6            (Phase 5)
│   ├── mail/                     # Mailer interface + net/smtp impl
│   ├── storage/                  # Storage interface + s3.go (presign, head, delete)    (Phase 5)
│   ├── database/
│   │   ├── db.go                 # pgxpool setup
│   │   ├── migrate.go            # embedded golang-migrate runner
│   │   ├── migrations/           # 000001_init_schema.up.sql / .down.sql ...
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
├── README.md  .editorconfig  .gitignore  .env.example
├── compose.yaml                  # postgres, mailpit, storage
└── Dockerfile  .dockerignore  Makefile  sqlc.yaml  .mockery.yml  .golangci.yml  go.mod
```

Folders are created in the phase that first needs them, not up front.

Three layers, dependencies pointing one way only: `router` → `handler` → feature packages (`auth`, `admin`, `family`, `contacts`, `event`, `media`) → `database/gen`. Feature packages hold the services (logic and permission checks) and know nothing about HTTP; `handler` holds all HTTP code; routes live only in `router/` (§6.1). Nothing imports `handler` except `router`, so there are no import cycles.

Why the embed package is not inside `web/`: Go tooling walks `./...`, and `node_modules` can contain stray `.go` files. Keeping Go code out of the Node tree avoids that.

Why `family/` is one package: relationship writes need the ancestor walk (cycle check) and the tree needs both people and relationships; splitting them into `person/relationship/tree` would make them import each other.

## 3. Database schema

Conventions: **Postgres 18**. UUID PKs via the built-in `uuidv7()` (Postgres 18, no extension): time-ordered, so B-tree indexes stay compact. `uuidv7()` embeds the creation time. That's acceptable: IDs reach signed-in members, and media IDs also appear in presigned bucket URLs (§9), which can be forwarded. Those keys already carry the upload year (`media/<yyyy>/...`), and anyone holding the URL can see the photo itself, so the extra exposure is only the upload timestamp. **Postgres 18 everywhere:** local compose, the CI test service, production, and the `pg_dump` client used for backups (an older `pg_dump` refuses a newer server). Managed hosting (§14 Option B) is only an option on providers that offer 18. `TIMESTAMPTZ` everywhere. Partial dates are stored as a date plus a precision flag (`1950-01-01` + `year` means "1950"). Soft delete via `deleted_at` on people, events and media.

### 3.1 `000001_init_schema.up.sql`

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;   -- fuzzy name search for tagging autocomplete

-- Authentication ------------------------------------------------------------
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT uuidv7(),
  email         TEXT NOT NULL UNIQUE CHECK (email = lower(email)),
  role          TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member')),
  person_id     UUID,                                  -- FK added below
  disabled_at   TIMESTAMPTZ,
  last_login_at TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE login_tokens (
  id          UUID PRIMARY KEY DEFAULT uuidv7(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  BYTEA NOT NULL UNIQUE,                   -- SHA-256 of a 256-bit random token
  expires_at  TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_login_tokens_user ON login_tokens(user_id, created_at DESC);

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
  CHECK (death_date IS NULL OR is_deceased)             -- a death date implies deceased
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
  kind              TEXT NOT NULL CHECK (kind IN ('photo','video')),
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

### 3.2 `000001_init_schema.down.sql`

```sql
DROP FUNCTION IF EXISTS masked_birth_date(persons, boolean, uuid);
DROP TABLE IF EXISTS media_tags, media, event_persons, events, contact_methods, relationships,
                     sessions, login_tokens, persons, users CASCADE;
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

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/healthz` | public | Liveness (process up) |
| GET | `/readyz` | public | Readiness (DB reachable) |
| POST | `/api/auth/magic-link` | public, rate-limited | `{email}` → always `202`, email sent only if user exists and is enabled |
| POST | `/api/auth/verify` | public, rate-limited | `{token}` → sets session cookie |
| GET | `/api/auth/me` | session | Current user, role, linked person |
| POST | `/api/auth/logout` | session | Deletes the session row |
| GET | `/api/admin/users` | admin | List users |
| POST | `/api/admin/users` | admin | Invite: `{email, role, person_id}` (person required, §5.1) |
| PATCH | `/api/admin/users/{id}` | admin | Role, disable/enable, link person |
| GET | `/api/tree` | session | All non-deleted people + relationships (flat; see §10) |
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

## 5. Authentication & sessions

**Model: invite-only.** Only emails that an admin has created in `users` can sign in. Nobody can self-register.

**Sign-in flow**

1. `POST /api/auth/magic-link {email}` always returns `202` with the same body, whether or not the email exists (no account enumeration). The email is sent **asynchronously** (background goroutine/queue) so response time doesn't reveal whether a message was sent.
2. If the user exists and isn't disabled: generate 32 random bytes, store `SHA-256(token)` in `login_tokens` (TTL 15 min), and email `https://<BASE_URL>/auth/verify#t=<token>`. New requests invalidate that user's older unconsumed tokens.
3. The link opens an SPA page; the token is in the **URL fragment**, so it never reaches server logs or `Referer`. The page POSTs it to `/api/auth/verify`. This also defeats email link-scanners that GET links and would otherwise burn a one-time URL.
4. Verify consumes atomically and re-checks that the user is still enabled (they may have been disabled after the link was sent): `UPDATE login_tokens t SET consumed_at = now() FROM users u WHERE t.token_hash = $1 AND t.consumed_at IS NULL AND t.expires_at > now() AND u.id = t.user_id AND u.disabled_at IS NULL RETURNING t.user_id`. On success create a session.

A 256-bit random token makes brute force moot, so no attempt counters are needed. Plain SHA-256 is appropriate for high-entropy secrets (it isn't for 6-digit codes, which is why V2's scheme was weak).

**Sessions**

- Cookie value: 32 random bytes, base64url. DB stores `SHA-256` of it (`sessions.id_hash`).
- Cookie flags: `HttpOnly; Secure; SameSite=Lax; Path=/`. Name `__Host-fh_session` in prod. In local dev set `COOKIE_SECURE=false` and drop the prefix, because Safari doesn't accept `Secure` cookies on `http://localhost` **(verify)**.
- Lifetime: 30-day sliding window, 180-day absolute cap. `last_seen_at` is only rewritten when it's more than an hour stale, to avoid a DB write on every request. Logout and "disable user" delete rows, so revocation is immediate.

**Rate limits** (in-memory `x/time/rate` is fine for a single instance): magic-link 5/hour per email and 20/hour per IP; verify 10/minute per IP.

**CSRF:** `SameSite=Lax` plus the stdlib `http.CrossOriginProtection` (Go 1.25), which rejects non-safe cross-origin browser requests using `Sec-Fetch-Site` and `Origin`; and require `Content-Type: application/json` when a body exists. **(verify it accepts the Vite-proxied dev origin without extra config; otherwise add `BASE_URL` with `AddTrustedOrigin`.)**

**Bootstrap:** `family-hub admin create --email you@example.com` creates the first admin (no env-var backdoor).

### 5.1 Linking a login to a person (so people land in the right spot)

An admin decides who each login *is*; nobody claims a spot themselves.

- **Invite flow:** the admin picks the person first (trigram search over the tree), or creates the person in the same step, then enters the email and role. Searching before creating prevents duplicate nodes. A person is **required**: every invited login is linked at invite time. (Only the bootstrap admin from `admin create` may start unlinked; they link themselves from the admin screen.)
- **Constraint:** `uq_users_person` allows at most one login per person. Couples who share an email address still need separate invites.
- **Guard:** soft-deleting a person who has an active login is refused until the admin unlinks or disables that login.
- **On login:** `/api/auth/me` returns `person_id`; the tree opens centered on that person with a "you" marker and a "back to me" button, and relationships are labelled from their point of view ("your aunt Rosa"). Labels are computed client-side from the already-loaded tree using the lowest common ancestor; build that as a pure function with a table-driven test (cousins once removed, half-siblings, step-relations).
- **Email changes:** admin edits `users.email`; the person link, and therefore tree position, is unaffected.

### 5.2 Contact info and visibility

Each person's contact details live in `contact_methods`, one row per phone/email/address/other, each with its own visibility.

| Rule | Behavior |
|---|---|
| Default | `hidden`. Nothing is shared until the person flips a toggle ("Visible to family"). |
| Who edits | The linked user edits their own rows via `/api/me/contacts`; admins can edit anyone's, including relatives with no login (admin-entered rows also default to `hidden`; the admin marks them `family` only with that person's consent). |
| Who sees a row | The owner and admins: always. Any other signed-in user: only rows with `visibility = 'family'`. |
| Enforcement | In the SQL/service layer (`WHERE visibility = 'family'` for non-owners), never in the frontend. Hidden values are not sent to the browser at all. |
| Where contacts appear | `GET /api/persons/{id}` and `GET /api/directory` only. They are **not** in `/api/tree`, `/api/persons?q=` search results, logs, or error messages, and search never matches on contact values. |
| Birthdays | **Month and day** are visible to every signed-in user. The birth **year** of a *living* person is visible only to admins and to that person themselves; for deceased people the full date is visible to all. Masking happens in SQL via the `masked_birth_date` function (§3.1), so the year never reaches a non-permitted browser, including through `/api/tree`. A living person with only a year or "circa" precision shows nothing to others. |
| Login email | `users.email` is an authentication detail and is **never** exposed. A contact email is a separate, optional row the person chooses to add. |
| Admin visibility | Admins can see everything, and the profile settings page says so ("Administrators can see all information"). |
| First login | A short onboarding step walks the user through their profile and shows each contact field with its visibility toggle, defaulting to off. |

Changes take effect immediately because nothing about contacts is cached client-side across sessions.

**Be upfront about admin access.** Hidden contact info is hidden from other *family members*, not from whoever runs the system: the admin account and anyone with database or backup access can read it. That's an honest and reasonable trade-off for a family project, but say so in the UI and the invite email ("Administrators can see all information you enter"). Lighter protection options, if you ever want them: keep the DB and backup buckets restricted to you, and never log contact values. Application-level encryption of address/phone values is possible but only protects against a stolen backup, not against the admin, so I'd skip it for now.

### 5.3 Upcoming birthdays (in-app only)

Birthdays are shown in the app; **no birthday emails are sent** (decided; see §16). The only email the app sends is sign-in and invite links.

- **Panel:** the home page shows living, non-deleted people with `birth_date_precision = 'day'` whose birthday falls in the next ~14 days, relative to "today" in `APP_TIMEZONE`. Feb 29 birthdays show on Feb 28 in non-leap years.
- **Masking:** same rules as everywhere (`masked_birth_date`, §3.1), so members see "Rosa, Friday" and never her age.
- **Where it lives:** one query in the `family` package; no scheduled job, no extra tables.
- **Mail deliverability (for sign-in links):** sending from a domain you control needs SPF, DKIM and DMARC records with your mail provider; set that up in Phase 4c and send a test to Gmail and Outlook before relying on it. Volume is tiny, so any provider's free tier works.

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
    RequireSession func(http.Handler) http.Handler // from auth
    LimitMagicLink func(http.Handler) http.Handler
    LimitVerify    func(http.Handler) http.Handler
    TrustedProxy   bool
}

func New(h Handlers, mw Middleware) http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    if mw.TrustedProxy {                // only when actually behind a proxy you control
        r.Use(middleware.RealIP)
    }
    r.Use(httpx.RequestLogger)          // JSON logs via log/slog
    r.Use(middleware.Recoverer)
    r.Use(httpx.SecurityHeaders)        // CSP, nosniff, Referrer-Policy, X-Robots-Tag: noindex

    r.Get("/healthz", h.Health.Live)
    r.Get("/readyz", h.Health.Ready)

    r.Route("/api", func(r chi.Router) {
        r.Use(middleware.Timeout(30 * time.Second))
        r.Use(http.NewCrossOriginProtection().Handler) // stdlib CSRF (Go 1.25)
        r.Use(httpx.RequireJSON)                        // Content-Type check when a body exists
        r.NotFound(httpx.NotFound)
        r.MethodNotAllowed(httpx.MethodNotAllowed)

        r.With(mw.LimitMagicLink).Post("/auth/magic-link", h.Auth.RequestMagicLink)
        r.With(mw.LimitVerify).Post("/auth/verify", h.Auth.Verify)

        r.Group(func(r chi.Router) {
            r.Use(mw.RequireSession)
            r.Get("/auth/me", h.Auth.Me)
            r.Post("/auth/logout", h.Auth.Logout)

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
            })
            // Phase 5: /events and /media routes added here the same way.
        })
    })

    r.Handle("/*", webui.Handler()) // SPA fallback; /api/* never reaches it
    return r
}
```

Role checks in the router (`RequireRole`) are the coarse gate; the fine-grained rules (a member may PATCH only their own person, only profile fields) live in the service, so they're enforced and tested regardless of routing.

Changes vs V2: no CORS middleware (Vite proxies `/api`, so dev is same-origin too); no compression middleware (Caddy compresses in prod); timeout scoped to `/api`; CSRF via the stdlib instead of a hand-written guard; JSON 404s; security headers; request body limits via `http.MaxBytesReader` in a JSON decode helper (1 MiB default).

### 6.2 Embedded migrations (`internal/database/migrate.go`)

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

func Migrate(databaseURL string) error {
    src, err := iofs.New(migrationsFS, "migrations")
    if err != nil { return fmt.Errorf("migrations source: %w", err) }

    // pgx v5 driver registers scheme "pgx5"; convert postgres:// -> pgx5://
    m, err := migrate.NewWithSourceInstance("iofs", src, toPgx5Scheme(databaseURL))
    if err != nil { return fmt.Errorf("migrate init: %w", err) }
    defer m.Close()

    if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return fmt.Errorf("migrate up: %w", err)
    }
    return nil
}
// import _ "github.com/golang-migrate/migrate/v4/database/pgx/v5"   (verify scheme name)
```

golang-migrate takes a Postgres advisory lock, so concurrent starts are safe. Run on startup only when `AUTO_MIGRATE=true`; otherwise use `family-hub migrate up`.

### 6.3 SPA handler (`internal/webui`)

```go
//go:embed all:dist
var distFS embed.FS

func Handler() http.Handler {
    sub, err := fs.Sub(distFS, "dist")
    if err != nil { panic(err) }
    files := http.FileServer(http.FS(sub))

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
        if p == "" { p = "index.html" }

        if _, err := fs.Stat(sub, p); err != nil {      // unknown path -> client-side route
            r.URL.Path = "/"
            w.Header().Set("Cache-Control", "no-cache")
        } else if strings.HasPrefix(p, "assets/") {      // Vite fingerprinted files
            w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
        }
        files.ServeHTTP(w, r)
    })
}
```

`all:` is required so a placeholder dotfile would still embed; the stub approach (§8.1) avoids needing one.

### 6.4 `main.go` shape

Subcommands are parsed with the stdlib `flag` package. `seed` loads the sample family (dev only; refuses to run unless the database is empty). `serve` (default): load config → pgxpool → optional migrate → build `App` → `http.Server` with `ReadHeaderTimeout`, `signal.NotifyContext` for SIGINT/SIGTERM → graceful `Shutdown(30s)`. `migrate up|down|version|force N` and `admin create` reuse the same config loader.

### 6.5 Configuration (env vars)

| Var | Example | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://app:app@localhost:5432/familyhub?sslmode=disable` | |
| `HTTP_ADDR` | `:8080` | |
| `BASE_URL` | `http://localhost:5173` (dev), `https://family.example.com` | Used in email links and the Origin check |
| `AUTO_MIGRATE` | `true` | |
| `TRUSTED_PROXY` | `false` | Enables `RealIP` |
| `COOKIE_SECURE` | `true` | `false` for local http |
| `SMTP_HOST/PORT/USER/PASS`, `MAIL_FROM` | `localhost:1025` (Mailpit) | |
| `S3_ENDPOINT` | `http://storage:9000` | What the **server** uses |
| `S3_PUBLIC_ENDPOINT` | `http://localhost:9000` | What the **browser** uses; presigned URLs are signed against this host |
| `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_PATH_STYLE` | `auto` / `true` for R2 and local servers | |
| `MAX_UPLOAD_BYTES` | `2147483648` | Single-PUT cap (§9) |
| `MEMBERS_CAN_CONTRIBUTE` | `false` | Opens event creation and media upload to members |
| `APP_TIMEZONE` | `America/New_York` | Defines "today" for the upcoming-birthdays panel |
| `BACKUP_PING_URL` | `https://hc-ping.com/<uuid>` | Pinged after each successful backup (§11) |

`S3_PUBLIC_ENDPOINT` matters because SigV4 includes the `Host` header: if the app runs in Docker and signs against `storage:9000`, the browser's request to `localhost:9000` fails signature validation.

## 7. Frontend

- React 19 + TypeScript + Vite; TanStack Query for server state; React Router.
- `vite.config.ts`: `server.proxy['/api'] = 'http://localhost:8080'`; `build.outDir = '../internal/webui/dist'`, `emptyOutDir: true`.
- Typed client: generate from an OpenAPI file later if the API grows; hand-written fetch wrapper for now with one error type.
- **Family tree view: a Workday-style focus view, not a canvas of the whole family.** The screen shows one **focus person** with their parents' household above, their own household beside them, and their children below: about two generations up and down. Siblings sit next to the focus person.
  - **Cards:** initials (photo once media lands in Phase 5), name, life dates (month/day only for living people to non-admins, §5.2), and a badge for what's collapsed, e.g. "3 children".
  - **Interaction:** tap a card to open a detail panel (bio, visible contacts, photos); tap an arrow to expand a branch in place; tap "Make focus" to re-center; "Back" and "Back to me" are always visible; a search box jumps to anyone. Pan and zoom aren't required.
  - **Households play the role of Workday's teams.** A couple, or a person with children and no partner on record, is one grouped card. Multiple marriages appear as separate households, with ended couples marked "former". Adoptive and foster links get a small label; step-relations are derived (§3.4) and labelled from the viewer's point of view. Households are derived from `relationships`, with no schema change.
  - **Layout:** only a few generations are drawn at a time, so plain CSS grid/flex with SVG connector lines is enough and no auto-layout engine is needed. Reach for elkjs or React Flow only if you later add a "whole family" overview.
  - **Data:** the client builds the view from the already-loaded `/api/tree` payload. A server-side "neighborhood" endpoint (§10) is a later optimization.
  - **Accessibility for older relatives:** large tap targets and type, high contrast, keyboard navigation, no hover-only actions, text labels instead of icon-only buttons.
  - **Prototype first:** build the focus view as a standalone clickable mockup at the start of Phase 4b, before the real tree, and show it to one non-technical relative. It uses no backend or database: the family data is hardcoded in the page, shaped like the `/api/tree` payload (§10) so the household-grouping logic can carry over into the real frontend. Real family shape and names are fine in the private mockup; anything committed to the repo uses made-up names. It is still the riskiest UI piece, but far smaller than a full-graph layout.
- Upload modal: request `upload-url`, `XMLHttpRequest`/`fetch` PUT with the returned headers (needed for progress events), then `confirm`. Compute width/height/duration client-side from the file for the confirm call.
- Auth pages: `/auth/login` (email form), `/auth/verify` (reads `#t=`, POSTs, redirects).
- **Member experience is read-mostly:** tree view centered on "you", person profiles, a directory of visible contact info, and a "My profile" page (bio, avatar, contact methods with visibility toggles). Tree-editing controls are not rendered for members.
- **Admin editing without raw edges:** admins never see "relationship type: parent_child, person A/B". From a person's card they choose "Add parent / child / partner / sibling", pick or create the other person, and the app maps that to a relationship (and, for "sibling", to shared parents). "Add parent" and "Add child" ask biological, adoptive or foster (default biological). "Add partner" asks **married or partner** and whether the relationship has **ended**; both can be changed later from the couple's card. Large icon buttons, plain-language labels, and undo (soft delete + restore) keep this safe for non-technical admins.

## 8. Tooling: Makefile, Compose, Dockerfile

### 8.1 Makefile (essentials)

```make
DATABASE_URL ?= postgres://app:app@localhost:5432/familyhub?sslmode=disable
export DATABASE_URL

.PHONY: up down dev-api dev-ui ui-stub generate test lint vuln build migrate-up migrate-down migrate-new

up:            ; docker compose up -d
down:          ; docker compose down

# Creates a placeholder so `go build/vet/test` work before the UI has been built.
ui-stub:
	@test -f internal/webui/dist/index.html || \
	  (mkdir -p internal/webui/dist && echo '<!doctype html><title>stub</title>' > internal/webui/dist/index.html)

# sqlc output + mockery mocks; both committed, CI fails if they're stale
generate:      ; go tool sqlc generate && mockery
migrate-new:   ; go tool migrate create -ext sql -dir internal/database/migrations -seq $(name)
migrate-up:    ; go run ./cmd/family-hub migrate up
migrate-down:  ; go run ./cmd/family-hub migrate down 1

dev-api: up ui-stub ; AUTO_MIGRATE=true COOKIE_SECURE=false go run ./cmd/family-hub
dev-ui:        ; cd web && npm run dev

test: ui-stub ; go test ./...
lint: ui-stub ; golangci-lint run && cd web && npm run lint && npm run typecheck
vuln:          ; go tool govulncheck ./...

build:
	cd web && npm ci && npm run build
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/family-hub ./cmd/family-hub
```

`sqlc`, `govulncheck` and the `migrate` CLI are pinned in `go.mod` via `go get -tool ...` (Go ≥1.24), so there's no `@latest`. mockery (v3) and golangci-lint are installed as pinned binaries instead, locally and in CI, because both projects advise against installing via `go get`/`go tool` (dependency conflicts) **(verify current guidance)**. `go tool migrate` is only used for `create`; `up/down` go through the app binary, so the CLI doesn't need database build tags **(verify)**.

### 8.2 `compose.yaml` (shape)

```yaml
services:
  postgres:
    image: postgres:18
    environment: { POSTGRES_USER: app, POSTGRES_PASSWORD: app, POSTGRES_DB: familyhub }
    ports: ["5432:5432"]
    volumes: [pgdata:/var/lib/postgresql]   # Postgres 18 images moved the data dir under here (verify)
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app -d familyhub"]
      interval: 5s
      retries: 10
  mailpit:                       # catches outgoing mail; UI on :8025
    image: axllent/mailpit
    ports: ["1025:1025", "8025:8025"]
  storage:                       # added in Phase 5: S3-compatible server chosen then (see §9.5)
    # image: ...
volumes: { pgdata: {} }
```

### 8.3 Dockerfile (shape)

```dockerfile
# Set to whatever toolchain go.mod requires (>=1.24 for `go tool`).
ARG GO_VERSION=1.25

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
# vite outDir is ../internal/webui/dist
RUN npm run build

# Dockerfiles don't support trailing comments on instruction lines, so comments stay on their own lines.
FROM golang:${GO_VERSION}-alpine AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/family-hub ./cmd/family-hub

# Pin the current Alpine release. Phase 6 adds ffmpeg + vips here.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=gobuild /out/family-hub .
EXPOSE 8080
HEALTHCHECK CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["./family-hub"]
```

`.dockerignore` (keeps `node_modules` and local build output out of the build context, so `COPY . .` doesn't drag in gigabytes):

```
.git
bin
docs
**/node_modules
web/dist
internal/webui/dist
*.md
.env*
```

The image builds its own `dist/` in the web stage, so excluding the local one is correct.

## 9. Media pipeline

### 9.1 Upload

1. `POST /api/media/upload-url {filename, mime_type, size_bytes, event_id?}`. Server validates the MIME against an allowlist (`image/jpeg`, `image/png`, `image/webp`, `image/heic`, `image/tiff` for flatbed scans of old photos, `video/mp4`, `video/quicktime`; TIFF and HEIC are shown through the Phase 6 JPEG variant, since browsers can't display TIFF) and `size_bytes <= MAX_UPLOAD_BYTES`; inserts a `pending` row with key `media/<yyyy>/<uuid>/original.<ext>` (extension derived from MIME, **never** from the filename); returns a presigned PUT (15 min) that signs `Content-Type` and `Content-Length`.
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

Nothing before Phase 5 stores files, so the MVP's local stack is just Postgres and Mailpit, and no Cloudflare/R2 account is needed until then.


MinIO was the V2 default. Its community edition is in maintenance mode and has been source-only (no new Docker images) since about October 2025. Candidates: a **pinned last MinIO image** (current lean: dev-only, so being frozen matters little), **Garage** (fallback; maintained, lightweight), RustFS (newer, less proven), SeaweedFS (heavier than needed), or no local server and a dev R2 bucket (loses offline dev). The spike runs the same checks against real R2 too, since R2's behavior is what matters in production. Whatever is chosen sits behind the `storage.Storage` interface, so the choice is cheap to reverse. The spike must prove: presigned PUT with signed `Content-Length`, `HeadObject`, presigned GET with `Range`, and browser CORS.

## 10. Family-tree queries

- **`GET /api/tree`** is two flat reads: all non-deleted `persons` (slim columns) and all `relationships` whose endpoints are both non-deleted. At family scale (hundreds to low thousands of people) this is a few hundred KB; no recursion needed. The client builds union nodes and runs the layout. Birth dates in this payload are masked per viewer (§5.2): non-admins get month/day for living people, never the year.
- **Optional later:** `GET /api/persons/{id}/neighborhood?up=2&down=2` returns just the households around one person (same masking), if the full payload ever gets too large for phones. Not needed at family scale.
- **One masked person read.** Every query that returns people to a browser (`/api/tree`, `/api/persons`, `/api/persons/{id}`, ancestors, descendants, directory, event and media tags) returns `masked_birth_date(p, $viewer_is_admin, $viewer_person_id)` (defined in the init migration, §3.1) and never selects `birth_date` or `p.*`. sqlc can't share column lists between queries, so the SQL function is the single place the rule lives, and a living person's birth year can't leak through an endpoint added later.
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

- Invite-only; enumeration-safe auth; single-use 256-bit tokens; server-side revocable sessions (§5).
- Security headers: strict CSP (`default-src 'self'`; `img-src`/`media-src` allow the bucket host), `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, `X-Robots-Tag: noindex`, HSTS in prod.
- All SQL via sqlc (parameterized). JSON body size limits. Upload MIME allowlist; keys never derived from user filenames.
- Secrets only from env/secret manager; separate S3 credentials for app vs. backup jobs; the app's key can't delete from the backup bucket.
- Contact info is private by default, filtered server-side per viewer (§5.2), excluded from the tree payload, search and logs, and never includes the login email.
- Family photos include children and living people: HTTPS only, no public buckets, no search-engine indexing, short-lived presigned URLs.
- Structured logs (slog) without tokens, cookies or presigned URLs.

## 13. Testing & CI

- **Go:** unit tests for services; integration tests against real Postgres (`familyhub_test` DB from compose, schema built by running the embedded migrations; truncate between tests). **Mocks:** mockery generates testify mocks (`.mockery.yml`, committed) for the interfaces at the edges and between layers: `Mailer` and `Storage` (into `internal/mail/mocks/`, `internal/storage/mocks/`), and the service interfaces declared in `handler` (into `internal/handler/mocks/`). The database is **never** mocked; query and permission behavior is tested against real Postgres. Tests are **table-driven** by default (a slice of named cases looped with `t.Run` subtests). Assertions use testify (`require` for setup, `assert` for checks). Time-dependent code (session expiry, rate limits, upcoming-birthday window) is tested with `testing/synctest` instead of sleeps. Specific tests: ancestry-cycle rejection (including a chain deeper than any display bound, and a loop closed through a foster link), ancestors/descendants exclude foster links, relationship canonicalization/uniqueness (one couple row per pair), third biological parent rejected (via POST and via PATCH changing a subtype), missing `subtype` defaults or returns 400, `ended` only on couples, death date requires `is_deceased`, search works for people without a last name, derived relations (full/half/unspecified siblings, including one sibling's second parent not yet recorded and people with more than two family parents, step-parents incl. former, step-siblings; adoptive links behave like biological; foster links yield only foster labels, e.g. two foster children of one couple are foster siblings, not full siblings), token single-use and expiry, session sliding/absolute expiry, rate limits, `confirm` rejecting wrong size. Permission tests are table-driven across roles: a member can't create/patch other people or touch relationships; a member can edit only their own contacts; hidden contact rows never appear in `/api/persons/{id}`, `/api/directory`, `/api/tree` or search for non-owners; the unique person-link constraint; refusing to delete a linked person; birth-year masking (living vs deceased, self, admin, across every person-returning endpoint: tree, search, profile, ancestors, descendants, directory, upcoming birthdays); upcoming birthdays: Feb 29 handling, year-end wraparound (Dec 28 → Jan 5), deceased/deleted people skipped.
- **Web:** Vitest + Testing Library for components; one Playwright smoke test (login via a test-only token → add person → see tree) once Phase 4b lands.
### 13.1 Branches, CI and releases

**Branches (trunk-based):** `main` plus short-lived work branches named `<type>/kdelpe/<descriptive-name>` (types `feature`, `fix`, `docs`, `chore`; e.g. `feature/kdelpe/magic-link-login`). Every change reaches `main` through a PR that is **squash-merged**, so `main` has one commit per PR. Releases are tags on `main`.

**`ci.yml`** runs on PRs into `main` and on pushes to `main`:

| Job | Checks |
|---|---|
| go | `go vet`, `golangci-lint`, `govulncheck`, `go test` with a `postgres:18` service |
| generated | Re-runs `sqlc generate` and `mockery`; fails if the committed output differs |
| web | Lint, typecheck, tests, build |
| docker | `docker build` (no push) |

**`release.yml`** is manual (`workflow_dispatch` with a `version` input such as `v1.2.0`) and runs **only from `main`**: the job checks `github.ref == 'refs/heads/main'` and runs in a `release` GitHub environment whose deployment-branch rule allows only `main`. Steps: fail if the tag exists → create the tag → build the image with the version injected (`-ldflags "-X main.version=..."`) → push to GitHub Container Registry → create a GitHub Release with generated notes. Deploying is a separate, deliberate step (the VPS pulls the tagged image).

**Workflow hygiene:** least-privilege `permissions:` per workflow (read-only by default; the release job gets `contents: write` and `packages: write`); third-party actions pinned to commit SHAs; Dependabot keeps them current.

**Repository settings (not in files):** `main` requires a PR (0 approvals, since the owner reviews their own PRs), passing CI checks once `ci.yml` exists, no direct pushes or force-pushes (enforced for admins too), no deletion. Squash merge is the only merge method enabled.

## 14. Deployment (Phase 4c options)

| Option | Shape | Pros | Cons |
|---|---|---|---|
| **A. Single VPS (recommended start)** | Docker Compose: app + Postgres (volume) + Caddy (automatic TLS); media on R2 | Cheapest, simplest, no cold starts; you own backups | You patch the box |
| B. Managed | App on Fly.io/Cloud Run + Neon/Cloud SQL/RDS (must offer Postgres 18) + R2 | Less ops | Cost, cold starts (Neon scale-to-zero), more moving parts |

Either way: `TRUSTED_PROXY=true` only when behind Caddy/the platform proxy, `COOKIE_SECURE=true`, `BASE_URL` set to the real domain, R2 CORS configured for it, and the §11 jobs scheduled before real family data is entered.

## 15. Phased roadmap

**MVP = Phases 1 through 4c**: a working tree, logins, contact visibility and upcoming birthdays, **deployed with backups and in the family's hands**. Photos and video come after, and the transcoding worker (Phase 6) only once you've actually hit a playback problem.

Real family data gets entered from Phase 4 onward, so the MVP ships (deploy + backups, Phase 4c) before any media work starts. Nothing precious should live only on a laptop.

| Phase | Goal | Exit criteria |
|---|---|---|
| **1. Foundation** | Repo, compose, config, migrations, health, CI, `webui` embed + SPA handler | `make dev-api` runs (it starts compose itself); `/healthz` + `/readyz` pass; CI green on a fresh clone (stub UI) |
| **2. Auth** | Invite-only magic link, sessions, rate limits, `admin create`, admin user endpoints, person linking (§5.1) | Full login via Mailpit; replay, expiry and disabled-user tests pass; one-login-per-person enforced |
| **3. People, tree & contacts API** | Admin-only persons/relationships writes, `/api/tree`, ancestors/descendants, cycle validation, contact methods with visibility filtering, `/api/directory` | Integration tests incl. cycle rejection and the role/visibility matrix (§13); seed script loads a sample family |
| **4a. Frontend shell** | Login UI, onboarding + "My profile" (bio and contact visibility toggles; initials only, avatars arrive in Phase 5), person profile page, directory, upcoming-birthdays panel | Log in via Mailpit and land on the home page; edit own bio and contact visibility; a member sees only visible contacts in profiles and the directory; works on desktop and phone |
| **4b. Family tree** | **Mockup first:** a standalone clickable focus-view mockup (§7) with the real family shape, tried on the owner's phone, then by one non-technical relative. Then the real focus-view family tree centered on "you" (§7), "back to me", search-to-jump, admin add-parent/child/partner/sibling flows (§7) | Sample family can be navigated by clicking, with multiple marriages and step-children shown as households, on desktop and phone; an admin can build a branch without touching raw edges |
| **4c Ship the MVP** | Deploy (§14), mail provider + domain with SPF/DKIM/DMARC, nightly backups + failure alert, restore drill, TLS, basic security headers | Deployed behind TLS; login email lands in Gmail and Outlook inboxes; restore drill passes; a deliberately failed backup triggers the alert; first real invites sent |
| **5. Events & media (originals)** | **Storage spike first:** choose the local server and prove the four capabilities in §9.5 against it and R2 with a throwaway Go test; add `storage` to compose. Then events + feed, upload/confirm/view (admin-only at first; members via `MEMBERS_CAN_CONTRIBUTE`), avatars (members can always set their own), tagging, orphan sweep, admin "missing info" tracker (§5.4) | Upload a 1 GB video from the browser and play it back with seeking |
| **6. Processing** | Worker: thumbnails, posters, H.264 variant, HEIC, EXIF | An iPhone HEVC `.mov` plays in Chrome and Firefox |
| **7. Harden** | Strict CSP covering the media bucket, runbook + handover note, export command if wanted (§11) | Second admin named and handover note written; CSP has no violations in normal use; restore drill re-run with media in the backups |

## 16. Decisions

**Decided.** These defaults favor the simplest option; each is easy to revisit.

1. **Roles:** admins maintain the tree, events and media; members edit only their own profile and contact info. One flag, `MEMBERS_CAN_CONTRIBUTE` (default `false`), later opens event creation and media upload to members.
2. **Visibility:** two levels for contact info (`hidden` / `family`). Birthday month and day are visible to all; the birth year of living people is visible to admins and that person only. Admins see everything.
3. **No birthday emails.** Birthdays appear only in the app (§5.3). The app emails nothing but sign-in and invite links.
4. **Access:** invite-only. No "request access" flow and no "suggest a correction" feature; relatives who spot a mistake message the admin.
5. **Video:** one 720p H.264 variant, built only when you hit a playback problem (Phase 6).
6. **Hosting:** Option A, a single VPS (§14).
7. **GEDCOM import/export:** later. Move it up only if you already have a tree file from another service.

**Still open, but not blocking until later**

- **Local storage server:** settled by the storage spike at the start of Phase 5 (§9.5).
- **Mail provider and sending domain:** needed before the first real invites, because sign-in emails depend on it (locally, Mailpit covers everything). My recommendation: register a cheap dedicated domain and use it for both the site and email, sending through a transactional provider with a free tier. Current lean: **Resend** (3,000/month free, SMTP), with Postmark as the switch-by-config alternative; register the domain at Cloudflare Registrar (DNS alongside R2), auto-renew on, several years prepaid, and listed in the handover note. **Verify current limits at Phase 4c.** Add the SPF/DKIM/DMARC records the provider gives you, then test delivery to Gmail and Outlook.
