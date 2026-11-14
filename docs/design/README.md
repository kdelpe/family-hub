# docs/design

The frontend design for the Pluviose-Monestime Family Hub, agreed in November 2026.

| File | What it is |
|---|---|
| `design-notes.md` | **The spec to build from:** colors, type, spacing, every page, interactions, animation timings, wording rules. |
| `pages/*.dc.html` | The mockup pages: **the visual target.** |
| `canvas.json` | How the pages were laid out on the design canvas (rows and order). |

The backend changes the design needs (access requests, invitation "from", stories, quick-look data, home page data) are folded into `docs/architecture-v3.md`; its §16 lists which ones are deferred.

## Building from these files

**The built pages match the mockups as closely as the browser allows:** same layout, copy, colors, type, sizes, spacing, radii and motion. `design-notes.md` is the behaviour spec and wins where the two disagree. Compare each page side by side with its mockup before calling it done, and agree any deliberate deviation with the owner first.

The page files were made on a design canvas and use that canvas's small runtime (`support.js`, `<x-dc>` markup, a `DCLogic` class), so they **won't run as-is** in this repo or a plain browser, and they aren't copied into `web/`. Read them for exact markup, copy, colors and sizes, and rebuild them as React components with the colors as CSS variables (`design-notes.md` §1).

Page order: before signing in → `Login`, `Request-Access`, `Welcome-Invite`, `First-Setup`, `Forgot-Password`. Signed in → `Home-Hearth`, `Main` (family tree), `People`, `Directory`, `Events`, `Stories`, `Story-Detail`, `Add-Story`, `My-Profile`, `Person-Profile`, `Manage-Access` (admins).

All people and details in them are made-up sample data. The repo is private, so they keep the family's surnames; if it ever goes public, scrub them first (see `CLAUDE.md`).
