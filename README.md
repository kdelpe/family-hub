# Family Hub

A private, invite-only family archive: an interactive family tree, person profiles, a family directory, and (later) events, photos and videos.

> **Status:** Phase 1 (foundation) in progress.

## Docs

- [Architecture & engineering plan](docs/architecture-v3.md)
- [Working rules](CLAUDE.md): branches, commits, approach

## Stack

Go (chi, pgx, sqlc) · Postgres · React + TypeScript + Vite · S3-compatible storage

## Local setup

Needs Go 1.27, Docker and golangci-lint 2.15.0 (`brew install go golangci-lint`, plus Docker Desktop).

```sh
make up      # Postgres and Mailpit in Docker
make test    # tests with the race detector
make lint    # golangci-lint, same version as CI
make build   # bin/family-hub
make down    # stop the containers (`docker compose down -v` also wipes the data)
```

| Service | Address | Login |
|---|---|---|
| Postgres | `localhost:5432`, databases `familyhub` and `familyhub_test` | `app` / `app` (local only) |
| Mailpit | http://localhost:8025 (catches all outgoing mail) | none |

Point GoLand's database tool, or `psql`, at the Postgres row above.
