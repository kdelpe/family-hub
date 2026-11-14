# Family Hub

A private, invite-only family archive: an interactive family tree, person profiles, a family directory, and (later) events, photos and videos.

> **Status:** the backend auth (Phase 2) is done; the frontend (Phase 4a) is starting with the pages before sign-in.

## Docs

- [Architecture & engineering plan](docs/architecture-v3.md)
- [Working rules](CLAUDE.md): branches, commits, approach

## Stack

Go (chi, pgx, sqlc) · Postgres · React + TypeScript + Vite · S3-compatible storage

## Local setup

Needs Xcode Command Line Tools (`xcode-select --install`; `go test -race` needs a C compiler), Go 1.27, Node 26, Docker Desktop and golangci-lint (`brew install go node golangci-lint`), then `cd web && npm ci` once. `make lint` requires the exact version in the Makefile's `GOLANGCI_LINT_VERSION`, which matches CI. If Homebrew has moved past it, bump that and the version in `ci.yml` together in a PR.

```sh
make up      # Postgres and Mailpit in Docker
make test    # tests with the race detector (needs `make up`)
make lint    # golangci-lint, same version as CI
make build   # the UI, then bin/family-hub with it embedded
make web-check # the UI's lint, type check and tests
make generate # sqlc code and mockery mocks (mockery 3.8.1 release binary on PATH); commit them, CI fails on stale output
make migrate-up   # apply pending migrations to the dev database
make dev-api # start Docker, migrate and serve on :8080 (the app, /health, /ready)
make dev-ui  # the UI with live reload on http://localhost:5173, sending /api to dev-api
make run     # serve on :8080 against the dev database, without migrating
make down    # stop the containers (`docker compose down -v` also wipes the data)
```

| Service | Address | Login |
|---|---|---|
| Postgres | `localhost:5432`, databases `familyhub` and `familyhub_test` | `app` / `app` (local only) |
| Mailpit | http://localhost:8025 (catches all outgoing mail) | none |

Point GoLand's database tool, or `psql`, at the Postgres row above.
