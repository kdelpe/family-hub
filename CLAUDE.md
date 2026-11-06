# Family Hub — working rules

A private, invite-only family archive. The design lives in `docs/architecture-v3.md`; read it before proposing changes.

## Current stage

**Planning. No application code yet.** Work right now is analysis and design documents only. Don't scaffold, generate or write code until the owner says the stage has changed.

Planned order:
1. Repo structure: settled (architecture §2).
2. Backend first: Phase 1 (foundation), 2 (auth), 3 (people/tree/contacts API on the §3 schema).
3. Then frontend; the clickable tree mockup opens Phase 4b. Code phase by phase (architecture §15), one short-lived branch and PR at a time.

## Guiding principle

**Essentials first.** Prefer cutting scope over adding it. Mark nice-to-haves as optional and leave them out unless the owner asks for them. No new dependency, service or feature without the owner agreeing first.

## Public repository

The repo is public. Security relies on auth, server-side checks and secrets in env vars, never on the code being hidden. Therefore:

- **No real family data in the repo**: seed data, fixtures and examples use made-up names and dates (a real family *shape* is fine).
- **No real domain, emails or personal details**: use placeholders such as `family.example.com`; real values live only in server env vars.
- **No secrets or `.env` files**: `.env.example` with safe local values only. GitHub secret scanning push protection stays on.

## Branches

Trunk-based: short-lived branches and `main`. No `develop` branch.

- `main`: reviewed and always working. Protected: changes arrive only by pull request, CI must pass, no direct pushes or force-pushes. Claude never commits to, pushes to or merges into `main`.
- All work happens on a short-lived branch off `main`, named `<type>/kdelpe/<descriptive-name>` in lowercase kebab-case, e.g. `feature/kdelpe/password-login-and-reset`, `fix/kdelpe/session-expiry`, `docs/kdelpe/schema-revision`, `chore/kdelpe/ci-workflow`. Types: `feature`, `fix`, `docs`, `chore`. One branch = one reviewable piece of work.
- The name says exactly what the branch does, usually 3–7 words (`add-docker-compose-postgres-mailpit`, not `foundation` or `setup`). If no short descriptive name fits, the branch is doing too much: split it.
- **Claude reviews before Copilot does.** When the work on a branch is done, Claude runs its own review (`/code-review`) of the branch's diff against `main`, fixes what it finds, and only then opens the PR (or pushes the final changes) for Copilot and the owner. Docs-only changes get the same consistency pass.
- Claude opens the PR into `main`; the owner reviews it and **squash-merges** it, so each PR becomes one commit on `main`. The small commits stay visible in the PR.
- Releases are manual, only from `main`: the owner runs the release workflow, which tags the commit (`vX.Y.Z`) and builds the image. Claude never triggers a release.

## Commits

- Small: one logical change per commit, easy to review in a few minutes.
- Every commit leaves the repo building with tests passing (once there is code).
- Message: **one line only**, a short imperative summary (≤ 72 chars). No body; the *why* goes in the PR description.
- **No AI attribution** anywhere: no `Co-Authored-By: Claude` trailer in commits, no "Generated with Claude Code" line in PR descriptions.
- Commit and push (to the work branch only) once the owner has okayed the step.

## How we approach work

- No separate design phase: `docs/architecture-v3.md` is the design. The PR description explains what changed and why, including any decision made along the way.
- HTTP code: routes are wired only in `internal/router`; all handlers live in `internal/handler` (HTTP only, one file per area); business logic lives in the feature packages' services (no HTTP). Dependencies point one way: router → handler → services.
- Items marked **(verify)** in the architecture doc are checked against current docs before relying on them.
- Keep the architecture doc current: when a decision changes, update the doc in the same change.

## Comments

- Names do the explaining. No comments that restate what a well-named variable, function or type already says.
- Comment only what the code can't show: *why* something is done, a non-obvious constraint, or a gotcha.
- Keep necessary comments to one or two lines. Exported Go identifiers still get a short doc comment where `golangci-lint` requires it, kept to one line where possible.

## To fill in later

<!-- Placeholders: complete these as the project takes shape. -->

- **Code style (Go):** _TBD_
- **Code style (TypeScript/React):** _TBD_
- **Testing conventions:** table-driven tests by default: a `tests := []struct{ name string; ... }` slice, looped with `t.Run(tt.name, ...)` subtests. testify `require` for setup/fatal checks, `assert` for comparisons. Mocks from mockery; the database is never mocked (real Postgres). Rest _TBD_.
- **PR review checklist:** _TBD_
- **Local setup / commands:** _TBD (see architecture §8 for the planned Makefile)_
- **Definition of done per phase:** _TBD_
