# Family Hub — working rules

A private, invite-only family archive. The design lives in `docs/architecture-v3.md`; read it before proposing changes.

## Current stage

**Planning. No application code yet.** Work right now is analysis and design documents only. Don't scaffold, generate or write code until the owner says the stage has changed.

Planned order:
1. Repo structure: settled (architecture §2).
2. Phase 0: a clickable tree-view mockup (no backend), tried on the owner's phone and then by a relative.
3. Then code, phase by phase (architecture §15), in small commits on `develop`.

## Guiding principle

**Essentials first.** Prefer cutting scope over adding it. Mark nice-to-haves as optional and leave them out unless the owner asks for them. No new dependency, service or feature without the owner agreeing first.

## Public repository

The repo is public. Security relies on auth, server-side checks and secrets in env vars, never on the code being hidden. Therefore:

- **No real family data in the repo**: seed data, fixtures and examples use made-up names and dates (a real family *shape* is fine).
- **No real domain, emails or personal details**: use placeholders such as `family.example.com`; real values live only in server env vars.
- **No secrets or `.env` files**: `.env.example` with safe local values only. GitHub secret scanning push protection stays on.

## Branches

- `main`: reviewed and stable. Claude never commits to, pushes to or merges into `main`.
- `develop`: where work lands. The owner reviews `develop` and merges it into `main` themselves (via PR).
- Larger pieces of work may use a short-lived `feat/<name>` branch off `develop`, merged back into `develop` when done.
- Pull requests into `main` come only from `develop`, and only after the CI checks pass.
- Releases are manual and only from `main` (a release workflow run by the owner). Claude never triggers a release.

## Commits

- Small: one logical change per commit, easy to review in a few minutes.
- Every commit leaves the repo building with tests passing (once there is code).
- Message: a short imperative summary line (≤ 72 chars), a blank line, then *why* if it isn't obvious.
- Commit only when the owner has okayed the step; never push without asking.

## How we approach work

- No separate design phase: `docs/architecture-v3.md` is the design. The PR description into `develop` explains what changed and why, including any decision made along the way.
- HTTP code: routes are wired only in `internal/router`; all handlers live in `internal/handler` (HTTP only, one file per area); business logic lives in the feature packages' services (no HTTP). Dependencies point one way: router → handler → services.
- Items marked **(verify)** in the architecture doc are checked against current docs before relying on them.
- Keep the architecture doc current: when a decision changes, update the doc in the same change.

## To fill in later

<!-- Placeholders: complete these as the project takes shape. -->

- **Code style (Go):** _TBD_
- **Code style (TypeScript/React):** _TBD_
- **Testing conventions:** table-driven tests by default: a `tests := []struct{ name string; ... }` slice, looped with `t.Run(tt.name, ...)` subtests. testify `require` for setup/fatal checks, `assert` for comparisons. Mocks from mockery; the database is never mocked (real Postgres). Rest _TBD_.
- **Review checklist for develop → main:** _TBD_
- **Local setup / commands:** _TBD (see architecture §8 for the planned Makefile)_
- **Definition of done per phase:** _TBD_
