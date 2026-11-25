# Family Hub: Frontend Design Notes ("Heirloom")

The agreed look and behaviour for the Pluviose Family Hub frontend (Phase 4a/4b and later), from the design canvas of November 2026. The page files in `pages/` are the visual reference; this file is the spec to build from. Where the two disagree, this file wins.

All names, dates, stories, phone numbers and addresses in the mockups are **made up** sample data.

**Continue with Google is dropped** (owner's decision, November 2026): leave out every Google button, divider and line of Google copy the mockups still show (Sign in, Welcome, Forgot password, the invite email in Members & invites, the sign-in line in My profile).

**Relations drop "Your"** (owner's decision, November 2026): since you're signed in, a relation is just its title, "Mother", "Half-sister", "Cousin once removed", and "You" on your own card, wherever the mockups still say "Your mother".

## Scope for the MVP

`docs/architecture-v3.md` §16 defers a few things this design shows. Until each one lands, build the page as drawn without it:

| Deferred | Where it shows | What the MVP does instead |
|---|---|---|
| **On this day** | Home | Home shows Coming up, New in the family and Story of the week, without the On this day card. |
| **Record now**, and uploads that resume in parts | Add a story, step 2 | Upload a file only, up to the 2 GiB cap, said in plain words. |
| Any phone video (needs the Phase 6 converter) | Add a story, step 2 | The drop zone says which files work before upload: *"MP4 videos and audio files (MP3, M4A, WAV). iPhone videos: share them as 'Most Compatible' or export as MP4."* A video the browser reports as HEVC/QuickTime is refused with that same line. |
| HEIC photos, TIFF scans and phone videos (need the Phase 6 converter) | Events, + Upload | The drop zone says which files work: *"JPEG, PNG or WebP photos and MP4 videos. Scans: save them as JPEG. iPhone: share as 'Most Compatible'."* Other files are refused with that same line. |
| Machine transcription and English translation | Add a story, step 4; the finish screen | The transcript is typed or pasted; no "Show English translation". The finish screen has no "Preparing the video" or "Making captions" steps and the story shows no "Processing…": it's ready when saved. |
| Dark mode | everywhere | Light theme only (§1.1 keeps the variables ready). |

---

## 1. Look and feel

Warm, archival, calm. Parchment ground, cream cards, one deep green accent, a serif for headings. No gradients, no emoji, no icon-only buttons.

### 1.1 Colors (light theme)

| Token | Hex | Used for |
|---|---|---|
| `--bg` | `#F6F1E7` | Page ground |
| `--surface` | `#FFFDF8` | Cards, inputs, panels |
| `--surface-soft` | `#FBF7EF` | Soft panels (tree area, household boxes, textareas) |
| `--brand-panel` | `#EFE7D8` | Left brand panel on sign-in pages; nav hover |
| `--nav-active` | `#E9E0CF` | Current menu item, segmented control track |
| `--border` | `#E2D9C8` | Card and panel borders |
| `--divider` | `#EFE8DA` | Lines inside cards |
| `--chip` | `#F1EADC` | Tags, name chips, "set aside" notes |
| `--accent-tint` | `#E6EEE9` | Selected states, "your household", success notes, logo circle |
| `--input-border` | `#CFC4AF` | Inputs and secondary buttons |
| `--line` | `#B8AC94` | Tree connector lines, ground line, dashed outlines |
| `--avatar` | `#D9CDB5` | Initials circles (main) |
| `--avatar-soft` | `#E5DBC8` | Initials circles (secondary) |
| `--text` | `#2B2620` | Body text |
| `--text-2` | `#4A433A` | Secondary text |
| `--muted` | `#6B6358` | Labels, hints, dates (passes 4.5:1 on all grounds above) |
| `--accent` | `#2F5D50` | Primary buttons, links, focus person, "You" |
| `--accent-hover` | `#1E3F36` | Link hover |
| `--switch-off` | `#8A8070` | Toggle track when off |
| `--warning-tint` | `#F6E3D4` | "Not found — check spelling" badge |

Illustration colors (trees only): branches `#8C7B5E`, roots `#C2B59B`, mountains `#DED3BD` (back) and `#C9BBA0` (front), grass `#A89878`, logo leaves `#8A6F4A`. Leaf placeholder palette: `#2F5D50 #8A6F4A #9DB3A8 #B59A86 #8E9A86 #A7A08C #C2B59B`.

White text only ever sits on `--accent` (buttons, the focus avatar).

**Dark mode is deferred.** When it comes back, follow the person's device setting, with a Light / Dark / Automatic choice in My profile. A tested warm-dark mapping exists: ground `#1C1915`, surface `#26221D`, soft `#221E1A`, border `#3A342C`, text `#EDE6DA`, muted `#A89E8F`, links/strokes `#8CC5B1` / `#7FB7A4`, button green `#3C7A67`. Build all colors as CSS variables from day one so dark mode is a swap, not a rewrite.

### 1.2 Type

- **Headings:** Fraunces (opsz 9–144, weights 500/600). The mockups load both fonts from Google Fonts; the app bundles them as `woff2` files instead (architecture §7), for the CSP and privacy. The type looks the same.
- **Body:** Source Sans 3 (400/600). Base size **17px**; never below 13px.
- Page title (h1): 40px Fraunces 500. Sign-in heading 44px.
- Section titles: 20–24px Fraunces 500.
- Small caps labels ("PARENTS", "FAMILY HUB"): 13px, uppercase, letter-spacing 0.08–0.12em, `--muted`.
- Line height 1.5 for paragraphs.

### 1.3 Shape, spacing, size

- Radii: 8px menu items · 10–12px cards and inputs · 14–16px panels · 18–22px hero cards · fully rounded (pill) buttons.
- Page padding 32–40px; gaps 12–32px; cards pad 14–24px.
- **Touch targets at least 44px tall**; primary buttons 48–52px.
- Pages are fluid: sidebar 220–260px, content takes the rest (`flex: 999 1 560px`), side columns 260–340px that wrap under the content on narrow screens.
- Canvas mockups are 1280 × 1260.

### 1.4 Accessibility rules (non-negotiable)

- Real `<button>`, `<a href>`, `<label>` + `<input>`. No clickable `div`s.
- Visible text labels on every action (no icon-only buttons); `aria-label` where the visible text isn't enough.
- Everything reachable and usable by keyboard; hover effects also appear on `:focus-visible`.
- Toggles are `role="switch"` with `aria-checked`; segmented controls use `aria-pressed`; tabs use `role="tab"` / `aria-selected`.
- **`prefers-reduced-motion: reduce` turns every animation off** (trees appear fully grown and still).
- Decorative SVG gets `aria-hidden="true"`; meaningful SVG gets `role="img"` and an `aria-label`.

---

## 2. Shared pieces

### 2.1 Left menu (signed-in pages)

- Top: logo mark (52px circle, tree glyph) **centered above** "Pluviose" (Fraunces 22px/600) and "FAMILY HUB" (13px small caps). The whole block links to Home.
- Items, in order: **Home · Family tree · Members · Events · Stories**. My profile moved to the account picture (below; owner's call, 2026-11-21).
- **Admin** section (admins only), under a thin divider and small "ADMIN" label: **Members & invites**.
- **Account picture** (owner's pick A of three, 2026-11-21, like Reddit's): your initials in a 40px accent circle at the top right of every signed-in page, in a row of its own so it never sits on a page's top cards (owner's call): the row overlaps the page's top padding, moving pages down 32px (`--account-bar`), which the tree and the Members book take off their window-sized height. Requests waiting for an answer (architecture §5.2) show as a count in a `--notice` dot on it, read out as *"Your account, 2 requests waiting"*; it rereads the count every minute. A click opens a panel sliding in from the right over a scrim: your name and **View my profile ›**, **Requests** (*Family asking for your details*, with the count), **Suggest a relative**, **Change password**, then a rule and **Sign out**. Esc or a click on the scrim closes it back onto the picture; Tab stays inside. Suggest a relative and Change password open the **Account** page (`/account`), which holds the Sign-in card (password only) and the Suggest card.
- Current page: `--nav-active` background, 600 weight, `aria-current="page"`.
- **Hover / keyboard focus:** `--brand-panel` background plus `box-shadow: 0 1px 3px rgba(43,38,32,.12)`, transition 150ms. The current page keeps its own color and only gains the shadow.

**Gliding highlight (owner's pick, 2026-11-22):** the current page's `--nav-active` background is one pill that glides (Motion layout spring) to the page opened; a page outside the menu, like Requests, has none.

### 2.2 Logo

Placeholder until a real logo exists: circle `#E6EEE9` with a `#2F5D50` outline, a small tree (trunk, two branches, top bud in green, two side leaves in `#8A6F4A`). Keep it simple and swappable.

### 2.3 Person cards

Initials avatar (circle, Fraunces), name (600), one line underneath with the relation **from the viewer's point of view** ("Mother", "Half-sister").

**No birthdays or dates in the family tree**: not on cards or chips. Dates appear only on profiles and the birthday lists (the Members book shows none either, §4.3), following the masking rules (living people month/day only, deceased full years, "c. 1935" for approximate dates).

### 2.4 Card hover (everywhere)
Every card-style item lifts on hover **and** on keyboard focus inside it: `translate: 0 -4px` plus `box-shadow: 0 12px 26px rgba(43,38,32,.12)`, 200ms. Use the `translate` property (not `transform`) so it never fights a card's entrance animation. Applies to: Home cards and Story of the week, tree cards (which also scale slightly), Members list rows, Events timeline cards and Media tiles, Stories cards/storytellers/featured story, profile family cards, admin request cards. Off with reduced motion.

### 2.4a Confirm dialog
An answer that can't be undone asks first in one shared dialog (`components/ConfirmDialog`, Radix Dialog + Motion; owner's pick from the Motion + Radix mockup, 2026-11-22): centred over a blurred `--scrim`, it springs in (scale 0.92 → 1 and up 14px) and fades out. A serif question, an optional note under it, an error in a `--warning-tint` box if the answer fails (the dialog stays open), then the quiet choice and the primary one, right-aligned. Focus starts on the primary choice and stays inside; Escape, the backdrop or the quiet choice cancel it (not while the answer is being sent), and focus goes back to the button that opened it, or, once answered, to the line saying what happened. With reduced motion or Pause motion (§6.4) it appears without the spring.

### 2.4b Toast
One note at a time, bottom centre (`components/Toaster`, Radix Toast + Motion, 2026-11-22): `--text` background, `--surface` text, an optional **Undo**, and a `--gold` bar that drains over 5 seconds. It springs up from below; hovering or focusing it pauses the countdown; it can be swiped down. A new note ends the one before as if it had run out. Radix announces it to screen readers.


### 2.4c Name card (owner's pick, 2026-11-22)

Pointing at a person's name with a mouse, after a quarter second, opens a small card (Radix Hover Card) under it: their initials, full name, how they're related to you (*You* for yourself; nothing when there's no link), and two chips, *Profile* and *In the tree* (the tree centered on them). Used on Events' *Who was there* chips and Home's *Coming up* and *New in the family* names. Touch and keyboard don't open it: the name itself is still the link to their profile.

### 2.5 Buttons

Primary: pill, `--accent` background, white 600 text. Secondary: pill, transparent with `--input-border`. Quiet: underlined text link in `--accent`.

**Every pill button** (including segmented switches like Timeline / Media, and links drawn as buttons) responds to hover and keyboard focus: `translate: 0 -1px` + `box-shadow: 0 4px 12px rgba(43,38,32,.16)`, 150ms; on press it settles back (`translate: 0 0`, `box-shadow: 0 1px 3px rgba(43,38,32,.12)`). Quiet text links and switches (`role="switch"`) don't lift. Off with reduced motion.

---

## 3. Before signing in (in this order)

All five pages share a two-column layout: **left brand panel** (`--brand-panel`), **right form** (max-width 420–460px, centered, fades up 14px on load).

**Brand panel**, top to bottom, all centered: logo + name stacked · the animated tree (§6.1) filling the middle · tagline in Fraunces 26px, breaking after "stories,": *"A private home for our family's stories, / photos and history."* · proverb in italic `--muted`: *"Sa w plante, se li w rekòlte." — What you plant is what you harvest.*

### 3.1 Sign in
- Heading **"Lakay se lakay"** (`lang="ht"`), subline *"There's no place like home."*
- Email; Password with **Show / Hide**; "Forgot password?" link beside the password label.
- **Sign in** (primary), then centered: *"First time here? **Request access**"*.
- Small centered line: *"Already invited? Use the link in your invitation email."*
- No separate "New here?" box: the line under Sign in is the only Request access entry point.
- Never say "Welcome back" (we don't know who's visiting). Optional later: "Welcome back" only if this device has signed in before (a localStorage flag, no name stored).

### 3.2 Request access
- Intro: *"Tell us a little about yourself and an admin will take a look. If they approve, you'll get an invitation by email."* (No family surnames, since in-laws and married relatives may not carry them.)
- Full name · Email.
- **"How are you connected to the family?"**: Pluviose family · Married into the family. (No "family friend" option, on purpose.) **Owner's call (2026-11-21):** the hub follows the Pluviose family only, so the Monestime side is no longer offered or shown anywhere: no Monestime option here, no side chip in the Members book (owner's call, 2026-11-21) and none but **Married in** elsewhere, and the family strip drops its *lines* chip. The data keeps the `second` side, so it could come back; people recorded on it count as family.
- **"How are you related?"** as one sentence: *I'm the [daughter ▾] of [name] and [name]*, with **+ Add another name** (up to 4). Relationship list: daughter, son, grandchild, sister, brother, niece or nephew, cousin, wife/husband/partner, mother or father, other relative. Names are **typed**, never suggested from the tree (that would reveal family names to strangers). Hint: *"Name a parent, grandparent or sibling if you can — it helps us find your place in the tree."*
- "Anything else? (optional)" textarea.
- **Send my request** → thank-you state: *"Your request is with the admins. If they approve it, you'll get an invitation email… usually within a few days."* + *"Know one of the admins? A quick call or text can speed things up."* Always the same response, whatever the email.

### 3.3 Welcome (from the invitation email)
- Banner with the inviter's avatar: *"**Nadège Pluviose** invited you to the family hub."* (Shows who the invitation is **from**, §5.3.)
- *"Welcome to the family, Darline"*; *"Choose a password to finish setting up your account for <email>."*
- New password with Show/Hide (shown by default here) and a 4-segment strength bar + label. No rules text up front; if the password is refused, show one plain-language error under the field (e.g. *"Use at least 8 characters"*, *"That password is too common — try a short phrase"*), linked with `aria-describedby`.
- **Set password and continue** → First-time setup.
- Small print: *"This link works for 7 days. Not Darline? Go to sign in."* (The "administrators can see everything" notice lives in the invitation email and First-time setup step 2, not here.)

### 3.4 First-time setup (3 steps, progress bar, Back / Continue / Skip for now)
1. *"Let the family see who you are"*: photo (optional now) + "A line or two about you".
2. *"How can family reach you?"*: phone, email, address and **social accounts** (WhatsApp, Instagram, …), each with a switch, **all start as "Only me"**; label flips to "Visible to family" (email, social) or "Family can ask" (phone, WhatsApp, address). Footer: admins can see everything.
3. *"Here's your place in the family"*: grandparents → parents → **You** highlighted; *"Something look wrong? Let an admin know and they'll fix it."* → **Go to my home page**.

Not drawn in the mockup, proposed with the real data (November 2026):
- It shows the signed-in person's own data. **Continue** on step 1 saves a changed story (an error keeps them on the step); **Skip for now** leaves it as it was. The switches save as soon as they're flipped, the same switch as My profile's.
- Step 2 lists the rows already on file (an admin may have added some), in My profile's order. With none: *"Nothing here yet. You can add a phone, email or address on My profile."*
- Step 3 leaves out a generation the tree doesn't have, and foster parents (architecture §3.4); with no parents it says *"Your parents aren't in the tree yet."* The family line names up to three living first cousins: *"Your cousins Maya, Fritz and Sabine are here too."* (the mockup's *"are on the Pluviose side"* went with the sides, §3.2), *"…and 4 more"* past three, and nothing with no cousins.
- Signed out, it goes to sign in; a login linked to nobody goes straight to Home.

### 3.5 Forgot password (side path from Sign in)
- *"Forgot your password?"* + email + **Send me a reset link** → *"Check your email. If that email is on our family list, a reset link is on its way. It works for 30 minutes."* + spam/ask-an-admin hint.

---

## 4. Signed-in pages

### 4.1 Home ("Hearth")
- Hero card: small caps day-part ("FRIDAY EVENING"), **"Good evening, Maya"** (52px), one personal line (*"Your mom's birthday is in 6 days, and Fritz added 12 photos from Thanksgiving."*), buttons **Open the family tree** / **See new photos**. Right side: the small family tree (§6.2) with a caption line under it.
- Three cards that rise in one after another (0.3s / 0.45s / 0.6s): **Coming up** (date tiles), **On this day** (two photos cross-fading every 5s + one line), **New in the family** (recent people, "See everyone"). On hover or keyboard focus inside, each card lifts 4px with a soft shadow (use the `translate` property, so it doesn't fight the rise-in animation's `transform`).
- **Refreshed hero and cards (owner's pick H1 of three Home layouts, 2026-11-21):** the hero is a soft gradient from `--surface` to `--accent-tint` (radius 22px), with a small tree drawn on its right (branches `--tree-branch`, leaves `--leaf-1` and `--leaf-3`; hidden on phones, where it would only push the cards down). Under the greeting, one line names the soonest birthday from Coming up: *"Mackenson's birthday is in 3 days."*, *"… is tomorrow."*, *"… is today."*, or *"Your birthday …"* for the viewer's own; no line when none is coming up. Each card's button for the rest of its rows sits in the card's header, right of the title: **See all** on Coming up, **See more** on New in the family. The account picture at the top right and the Requests page come in their own PR. **Living tree (owner's pick, 2026-11-22):** once per visit the branches draw themselves in, one after another, then the leaves bud with a small spring; after that each leaf drifts up and down 2.5px on its own slow cycle (5–7s). Coming back to Home in the same visit shows it grown. Reduced motion shows it grown and still; there's no pause button (owner's choice, below).
- **Until photos and stories arrive (Phase 5), owner's calls (November 2026):** only **Coming up** and **New in the family** are built, side by side (On this day slots in between later), and the hero's personal line is the next birthday (above). Coming up lists birthdays from today through two weeks out, soonest first with its date tile in the accent tint: *"Nadège's birthday"* and *"Today"*, *"Tomorrow"* or *"In 6 days"*; with none, *"No birthdays in the next two weeks."* New in the family lists the people added in the last 30 days (up to 20), each with their relation to you and when: *"Cousin · added Tuesday"* (*"Added today"* when the tree doesn't connect them to you); the mockup's *"bio added"* lines need an activity log and are left out; with none, *"Nobody new in the last month."* Each card shows its first three, then a button (in its header, above) that opens the rest in place (owner's call, 2026-11-20, after a 5,200-person test family put about 190 birthdays in two weeks); New in the family's **See everyone** link is left out, since People is in the menu.
- **Story of the week** (full width, rises in at 0.75s, same hover lift): video still with play button and length on the left; on the right "STORY OF THE WEEK", title (Fraunces 32px), *"Told by Grandma Rose · recorded October 2026"*, a one-line quote from the transcript in Fraunces 19px, **Watch the story** / **More stories**. Driven by `stories.featured_at`; swappable later for "Photo of the week" or "Today in family history".

### 4.2 Family tree (focus view)
Built client-side from `/api/tree`.

Look:
- A **family strip** on the left of the search's row: *38 people · 5 generations · Earliest record: c. 1900, Jacmel* (pill chips).
- The tree sits on a `--surface-soft` panel (radius 22px). The mockup's pale watermark tree behind the rows is left out (owner's call, 2026-11-20, after trying it in a prototype): names didn't sit on its branches.
- **Even proportions:** every person, grandparents and grandchildren included, uses **the same card**: 190 wide (owner's call, 2026-11-21, so the tree fills its panel; 150 on phones), radius 14, a 50px framed portrait (cream + sand double ring) that fades between initials and a photo placeholder (6s, each person offset), name 15px/600 on one line (wrapping only on phones), relation 13px. Same gap (14px) between cards in every row and the same spacing between generations.
- Each generation has a small uppercase label above it (*Grandparents · Your parents · Your household & siblings · Your children · Grandchildren*). Couples and parent groups sit in a soft rounded group (`rgba(255,253,248,.75)`, 1px border); your household uses `--accent-tint` with a 2px accent border.
- **▲ / ▼ arrows sit on the panel's top and bottom edges** (owner's calls, 2026-11-21, replacing the pill under each grandparent or grandchild card): they name the family that way (*▲ Jean-Robert's parents*, *▼ Naïka's grandchildren*, or *Back to Jonas* where an arrow only walks back), and show until the tree is moved by hand (a drag, a pinch, Ctrl + scroll); **Recenter** brings them back. ▲ goes one generation up, to a parent: one whose parents' line continues if there is one, else any with parents, since the grandparents are always just above the panel's top; ▼ one down (to the child whose children have children); after an arrow, the other one walks back along the same people, even where nothing lies beyond, until a card, the search or Back to me starts afresh. Other branches are a click on their card away. The accessible name starts with the words shown, *"Jean-Robert's parents: center the tree on Jean-Robert Pluviose"*, so voice control finds it. They sit on the borders, half in and half out, so inside they cover only the empty top of the line down to the parents and the faded bottom edge; a long name is cut short with …
- **Connector lines** (`--line`, 2px, rounded corners) join each set of parents to their children: grandparents to the parent they raised, parents to you and each sibling, your household to each children group, a child to their own children. Lines from the same parents share one joining line just above the row they feed. `aria-hidden`; the generation labels say the same in words.
- Your children are grouped by their other parent (*"With Nadège"*, *"With Guerline"*), so each marriage reads as its own household; the label shows only when there's more than one group or partner.
- Adoptive and foster links get a small word after the relation: *"Son · adopted"*, *"Mother · adoptive"*, *"Daughter · foster"*, on every row, grandparents and grandchildren too. It's left out only when the relation already says it (*"Foster mother"*); a closer tie keeps it (*"Grandmother · foster"*). With no relation to the viewer, the card says *"Foster parent"* or *"Foster child"*, and a sibling only by fostering *"… · foster sibling"*.
- **Lines draw in** once both their ends have landed (0.4s each), from the parents down to the children, with the sign-in tree's drawing trick (§6.1).
- **The chain turns green:** while a card is lifted (hover or keyboard focus), the lines between that card and the focus person turn `--accent` and 2.5px, fading in over 150ms: a grandparent through their child, a sibling through the parents, a grandchild through their parent. A partner shares the household, so nothing lights. (Owner's call, 2026-11-20.)

One **focus person**; shows:
- **Grandparents** (two generations up), grouped per parent, with **▲ more** at the panel's top when there's more family above.
- **Parents** household.
- **Focus row:** siblings split left and right · the focus person's **household** box (`--accent-tint`, 2px accent border) with current partners, and former partners with a dashed border, labelled with your relation to them ("Former stepmother", "Mother"), or "Former partner" when there is none.
- **Children**, then **grandchildren**, with **▼ more** at the panel's bottom when any of them has children.
- Half-siblings labelled from the viewer's point of view ("Half-sister", or "… · half-sibling").

Interaction:
- **Every card has a View profile › link** under the relation (13px, 600), so there's no pop-up to open first (owner's call, 2026-11-21, replacing the mockup's quick look with its story lines, shared contact and Center here). Its accessible name adds the person's name, *"View profile of Joseph Pluviose ›"*. The card grows to 150px tall to hold it.
- **Hover or keyboard focus** on a card lifts it (`translateY(-5px) scale(1.07)`, soft shadow).
- **A click or tap anywhere else on the card** centers the tree on that person (owner's call, 2026-11-21: with no quick look, it no longer needs a double-click).
- Centering animates, and so does opening the page: the generations arrive one after another, top to bottom, 80ms apart and 0.4s each. They slide **down** into place when moving to a descendant, **up** when moving to an ancestor, and fade/scale slightly sideways otherwise, as on opening (owner's call, 2026-11-20). With reduced motion everything is simply there, the green chain included.
- No visible page title, trail of names or **Back** (owner's call, 2026-11-21: more room for the tree; a visually hidden `h1` stays for screen readers). The family strip sits on the left of one row; the search and **Back to me** on the right.
- **Zoom** (owner's call, 2026-11-21): only the tree zooms, inside its panel, which keeps its size so the rest of the page never moves. A − / 100% / + / **Recenter** control floats in the panel's top-right corner (50–150%, steps of 10%; the percentage resets the zoom and position, Recenter only the position, and is disabled until the tree is moved; both glide the tree back over 0.3 s, at once with reduced motion, and a move by hand takes over from where the tree is shown part way); a two-finger pinch on the tree (the page itself doesn't zoom there) and Ctrl + scroll or a trackpad pinch on computers zoom too, keeping the point they're made at in place. The cards and lines scale together.
- **The tree moves within its panel** at any zoom (owner's call, 2026-11-21): drag it with a mouse from anywhere, cards included (past 5px it's a drag, so the card isn't also centered on); on touch, one finger moves it any way and two fingers move and pinch it; a plain scroll of the wheel or trackpad moves it too (where the page doesn't scroll), and so do the arrow keys from a card or link in the tree, where + and − zoom. The panel's bottom edge fades softly where the tree runs on below it. At least 60px of it always stays in the panel. Resetting the zoom, or centering on anyone (even the person already centered, as **Back to me** does when it's you), brings it back to the middle; the zoom itself holds while centering on others, until the page is left.
- Search box suggests up to 6 matching names as you type; picking one centers the tree.
- Tip text in the panel's top-left corner, opposite the zoom control (on phones, just above the panel, since the corner is the zoom control's), in `--text-2` (owner's call, 2026-11-21): *"Click or tap a name to center the tree on them · drag to move it"*.
- An `aria-live` region announces "Family tree centered on …".
- The tree opens centered on you, or on the person a profile's **Center the tree on …** asked for (`/tree?person=<id>`), if they're in the tree. **The page fits the window and doesn't scroll** (owner's call, 2026-11-21): the tree's panel fills the height the family strip and search leave. On a window too short (under 520px) or narrow (under 800px, with the menu above the page; the panel is then the window's height less 140px, 440–900px) for that, the page scrolls, and a plain wheel and an up-and-down swipe scroll it, as the way back to the controls; dragging, Ctrl + scroll and pinching still move and zoom the tree. The panel's top edge starts where the grandparents' cards end, so nothing shows cut off there: the lines come down from the edge to the parents, then the household and siblings, and on down as far as the window allows (with no grandparents shown, the tree starts at its own top). The line down to the parents is 24px longer than the rest, so their label clears the ▲ arrow at every zoom. Zoomed in on a short window, where the person centered on would fall below the panel, the tree moves up just enough to show their card; where the page scrolls, keyboard focus landing on that card (after a search, say) scrolls the window to it too. The tip in the panel's corner (*Click or tap a name…*) goes once the tree is moved, as the ▲ / ▼ more arrows do, so it never sits over the cards, and comes back with Recenter.
- Later: keyboard arrows and phone swipes to move a generation; URL per focus (`/tree/<person>`) so browser Back works and links can be shared.

### 4.3 Members (route and file: `Members`)
**One page for everyone.** Members replaces both the old People grid and the Directory/Address book, so there is no second list of people anywhere. It's a green clothbound book, **one person per page, two per spread**, with the living A to Z and an **In Memory** section at the back for those who have passed.

**Page order** (index 0 is the empty space left of the closed cover):
1. **Front cover** (closed book, centered on the page). Leather-look green with a soft highlight, a darker hinge strip on the spine side, a double gold border with corner flourishes, the gold tree mark, **Pluviose**, a gold rule with a diamond, **FAMILY MEMBERS** in spaced capitals, and *Lakay se lakay* at the bottom. A light sheen sweeps across it every 7 s, and every 6 s the cover lifts slightly (rotateY −14°) to invite a click.
2. **Inside front cover:** dotted green endpaper with a bookplate: tree mark, *This book belongs to* / *The Pluviose Family* (owner's call, 2026-11-21: the note under it is gone, leaving room for a bigger logo later).
3. **Welcome** (page 1; owner's call, 2026-11-21, from three options): *Everyone in our family, in one book*, then three numbered steps, each a short title over one line: **Turn the pages** (*Click or drag a page to turn it, or tap or slide along the tabs on the edge.*), **Say hello** (*Email and social accounts appear when someone chooses to share them.*), **Ask for the rest** (*Phone numbers, WhatsApp and addresses are never printed here. Ask, and they decide.*). At the foot, clear of the page number: *Your own page comes from* **My profile ›**. No intro paragraph.
4. **Inside** (page 2): contents with dotted leaders and page numbers (Welcome · Members, A to Z · In Memory), every entry clickable. No letter buttons (the tabs on the edge do that) and no counts (of the living, the remembered or who married in) (owner's call, 2026-11-21): the book shows no family sides at all.
5. **Members A–Z**, one page each, by first name then last name (*Anna* before *Anna Maria*): big corner letter, portrait, name, relation (no **Married in** chip: owner's call, 2026-11-21), Email button, social pills, and the dashed **Request phone number / WhatsApp / address** buttons (become *"✓ … requested"*). Your own page shows *"This is your page"* + **Edit in My profile ›** instead of contact info. Someone without an account shows *"Not on the hub yet."* **Open profile ›** just under the name and relation, above the rule (owner's call, 2026-11-21); memorial pages put it in the same place. A long list of contacts scrolls inside the page, below the rule. **No dates.**
6. **In Memory** divider: an olive-branch drawing, *In Memory*, *Those who came before us…*, and the count.
7. **Memorial pages**, one each: soft parchment wash at the top, portrait in grayscale with a hint of sepia, name, relation, side, a leaf + **IN LOVING MEMORY**, one remembered line in italic quotes, then **Open profile ›**. No contact details and no dates.
8. A **Notes** page if needed to even out the pages, the **inside back cover** (endpaper with *Sa w plante, se li w rekòlte.* / *What you plant is what you harvest.*), then the **back cover** (closed, centered, small gold tree).

**Opening and closing:** click the cover, drag it, or press **Open ▶**. While closed, the book is shifted half a page and scaled up to fill the space (`translateX(calc(-30.5% + 8.54px)) scale(1.22)`, top-center origin, 172 px extra bottom margin so the buttons move down; 0.75 s), so the larger cover sits in the middle, and the green board, spine line and tabs are hidden. They appear as it opens. The back cover works the same in reverse. A cover is the size of the green board (owner's call, 2026-11-21), reaching past the pages at the top, bottom and outer edge as a hardcover does, so closed it hides the pages and their edges; it turns as one stiff board, its inside showing the endpaper within a green rim. Pages lie flat until they turn: no lifted corners (owner's call, 2026-11-21). The bottom corners are still click targets for turning.

**Turning:**
- **Click** anywhere on a page (not a button or link): the right page goes forward, the left page goes back.
- **Drag:** grab a page and pull it toward the spine. The page follows the pointer (`angle = acos(1 − 2p)`) and **bends**: it's two hinged pieces, with the outer 32% rotating an extra `sin(angle) × 30°`. It darkens toward vertical, the outer piece catches a highlight, and shadows fall on the page underneath and on the landing page. Let go past halfway, or flick toward the spine (measured over the last 100 ms, so a page held still isn't flicked), and it finishes; otherwise it falls back (200–660 ms), as it does when the touch becomes a scroll. A 6 px dead zone separates a click from a drag. On touch, use `touch-action: pan-y`.
- Also: a click on a page's bottom corner (the corners lie flat, nothing shows there), **◀ Previous / Next ▶** (0.9 s turn), **← →** when the book has focus, the **letter tabs** on the right edge (one per letter that has someone, plus **♡** for In Memory; the current one is highlighted), and the contents page. Tapping a tab or a contents entry flips straight to it in one turn. The book stays at that person, not just their spread, and a button on the page turned away hands keyboard focus to the book, so the arrow keys keep turning.
- **Slide along the tabs:** press on the tabs and slide up or down (mouse or finger; `touch-action: none` on the tab strip). After 6 px of movement it switches to scrubbing: the nearest tab widens to 46 px, a round accent bubble with that letter pops out on the page side of the tab, and the book jumps instantly (no turn animation) to that letter's spread as you pass each tab. Letting go leaves you there. A press without sliding acts like a tap (animated turn). Keyboard Enter/Space on a tab still works.
- **Page edges** pile up on the left as you go and thin out on the right.
- **Status line** under the book: *"Pages 3–4 · Darline & Fabienne"*; on the cover, *"Click the cover, or drag it, to open the book"*.

**Book / List** switch: the List shows **With us** and **In memory** as two 3-column grids (avatar, name, relation · city or "In memory"), and clicking a row opens the book at that page. **Find someone…** searches everyone and opens the book at the first match.

**Implementation:** one registered CSS property (`@property --fa`, a number in degrees) on the pages container drives the rotation, bend, shading and shadows through `calc()`, `sin()` and `cos()`. While dragging, write `--fa` straight to the element with no re-render per pointer move; animated turns are keyframes on `--fa`, and letting go is a CSS transition on `--fa`. Keep conditional elements (tabs, leaf) *after* the pages container in the markup so it is never re-created mid-drag. A library such as StPageFlip is fine if it keeps the same feel. With reduced motion: no sheen or cover nudge, and turns are instant; a dragged page doesn't follow the pointer, and letting go turns it at once, or not.

**Data:** living members come from `GET /api/directory` (every living person, with `has_account` for *"Not on the hub yet"*); the In Memory pages come from `/api/tree`'s persons with `is_deceased` (name, side, `bio_excerpt` as the remembered line, left out when there's none); relations come from the tree. Contact rules are in architecture §5.2.

**Built (owner's calls, 2026-11-21):**
- **Just the book** (owner's call after the first build): no page title, subtitle, **Find someone…** search or **Book / List** switch; the letter tabs and Contents do the finding. A hidden "Members" heading stays for screen readers. The book and the buttons under it **fill the window without scrolling**: the page height is the window less the buttons (440–820 px; on phones 440–700 px under the menu). On the shortest books, whatever doesn't fit on a page (the Welcome notes, a long remembered line) scrolls inside that page. Closed, the cover slides to the middle at its own size rather than the mockup's 1.22× scale, which would have made the open book shorter. No Pause motion button.
- **Under the book** (owner's call): closed at the front, there's only the line saying how to open it, no buttons (the cover opens by a click, a drag or **→**). Closed at the back (owner's call, 2026-11-21), there are **◀ Previous** and **↺ Start again**, the same pair of buttons: Previous opens the back cover to the last pages; Start again riffles back to Welcome page by page, as a hand would flip it in person, too quickly to read the pages going by (owner's call): the turns share 0.7 s, each no slower than 0.47 s and no quicker than 0.05 s, one straight after another, so a 17-spread book takes about 0.9 s, clicks on the way ignored; with reduced motion it goes straight there. Nothing moves by itself. They go as soon as the book starts to close (Previous, Next, a click, or a drag let go to finish), and come back as it starts to open. Open, the status line becomes a **page box**: *◀ Previous · Page [ 4 ] of 30 · Next ▶*. The two buttons are a matching pair (owner's call, 2026-11-21): the same size and the same outlined light style, neither one green. It shows the page the book is open at: the one typed (or jumped to), from the moment Enter is pressed, even when it's the right-hand page; after ◀ Previous or Next ▶, the spread's first page. Type a page number and press Enter (or leave the box) to turn straight there in one turn, and Escape or a number that isn't a page puts it back. The names in the status line are still announced to screen readers.
- **Page setup** (owner's call, 2026-11-21, from a test of three styles, then of palettes): no ruled lines. Every paper page has a thin double border inset 7px from the edge (2px between the lines), like a printed address book, drawn in see-through ink so both lines show on any tint. The tint fills the frame; the margin outside stays white paper. Pages take eight warm neutrals from coolors' popular neutral palettes in turn (`--book-page-1` to `-8`: papaya cream, desert sand, eggshell, tea green, almond, wheat, champagne pink, pale sage), ordered so each sits well beside the next and facing pages never match. The order starts at Welcome, so it's papaya cream beside the endpaper's light green. The letter tabs on the edge take the same eight in turn (owner's call, 2026-11-21), so no two tabs side by side match; the current tab is white. No greys, and no near-whites, as the page itself is white. The In Memory divider is olive sage (`--book-divider`); memorial pages keep a darker frame. On the tints, the grey secondary text uses `--text-2` so it stays readable. Every line on a page (the rule under the name, the dashed box, the contents' dotted leaders) is see-through ink too, never a cream or white, so none disappears on a light tint. The **Request …** buttons, see-through elsewhere, get the social pills' soft cream fill in the book, with an ink dashed outline, so they don't clash with the tint (owner's call, 2026-11-21). The corner letter and page number sit inside the frame.
- **Quick clicks** (owner's call, 2026-11-21): clicking again while a page turns queues another turn rather than being ignored, so four clicks turn four pages; queued turns run at twice the speed, back to back, and a click the other way turns back. Up to eight are queued.
- **Arrival** (owner's call, 2026-11-21, from a test of four openings): the book and its buttons sit in the middle of the window, a little lower than centered on desktop (44px above, the page 16px shorter than the window allows). On arrival, and whenever **Members** in the menu is clicked (even while already there), the closed book is picked up off the table: it starts lying flat and faded, and tilts up to face you over 1 s (`rotate: x 62deg`, 40px lower, 90% size, to upright), then the cover opens itself to the first pages. Meanwhile the line saying how to open it is kept out of sight (hidden, keeping its space). Not with reduced motion (it stays closed, saying how to open it). Going back to the cover with **◀ Previous** or a click just shows the cover and the line: the animation never replays then.
- **The covers swing flat** (they're board, not paper, so they don't bend like the pages), with no shadow cast where there's no page beside them. The green board inside a cover arrives as the cover lands and goes as it lifts, and the letter tabs fade in after it opens and leave as it starts to close.
- **Phones** turn **one page at a time**: the same book, cover and turns, one page filling the width, without the blank and Notes pages; dragging either way turns, and turning back brings the previous page in from the left. A tap anywhere on the front cover opens the book, and on the back cover goes back. Under the book, ◀ and ▶ show without their words and the page box without its label, so all three fit; the page box counts only the pages a phone shows.
- **The green board, the cover's sheen and its lift are on**, as described here. (The mockup's CSS leaves its reduced-motion block unclosed, so it showed them only with reduced motion; that was a slip.) The sheen, the lift, the peeking corner and the portrait fades stop with reduced motion or the site-wide Pause motion (§6.4); turns are instant with reduced motion.
- **Left out for now, to come back:** the **city chip** (there's no city field yet) and **Share a memory ›** (until Add a story exists).
- As elsewhere, relations drop "Your", and your own page says "You".
- **Ask to share** follows the Directory's rules it replaces: dashed **Request …** buttons become *"✓ … requested"*; once shared, *"✓ Phone number shared with you"* links to their profile, where the value is shown; admins, who can't ask, see *"Phone number and address on request"*. WhatsApp counts as a phone number. *"Not on the hub yet"* shows only when someone without a login has nothing shared either.
- The page edges stop growing at 18px, since a big family has thousands of pages. No library: the CSS-property approach above.

### 4.5 Events
Title **Events**. Segmented **Timeline / Media**, plus two buttons shown to admins (and to members once `MEMBERS_CAN_CONTRIBUTE` is on):
- **+ New event** opens an inline panel: *What happened?*, *Where?*, *When?* with a precision switch (**Exact day · Month · Year · About**, matching the date + precision columns), *Tell the story* (optional), *Who was there?* (person chips + Add person), Cancel / **Save event**. Saved events appear at the top of the timeline.
- **+ Upload** opens an inline panel: drop zone (*"Drag photos and videos here"* + **Choose files**), the cap in plain words (*"Photos and videos up to 2 GB each. Recording a longer interview? Add it as a story instead."*), **Add to event** picker (or *No event — just add to Media*), then a progress row per file (thumbnail tile, name, bar, "64% · 1.1 of 1.7 GB").
- **Timeline:** year column, a dot on a vertical line, event cards with title, date (precision-aware: "c. 1962"), story, photo strip, tagged-people chips.
- **Media:** 5-column grid of photos **and videos**; videos show a play button and their length. Captions under each.
- **Built ahead of media (2026-11-21):** the timeline only. Each card shows its title, the date and place on one line, the story and who was there, with each name linking to their profile.
  - Hidden until Phase 5 brings media: the Timeline / Media switch, **+ Upload**, the photo strip and the photo count.
  - Dates are written "November 13, 1984", "July 2025", "1984" and "Around 1962", with "c. 1962" in the year column.
  - An undated event has an empty year column. Undated events come last.
  - On phones, the year sits above each card and the line runs down the left edge.
- **Removing an event (admins, 2026-11-22):** a × at the top right of each card (shown on hover or focus; always on touch screens) asks *"Remove “…”?"* with *"It leaves the timeline for everyone. You'll have a few seconds to undo."*, **Keep it** / **Remove** (§2.4a).
  - The event slides out at once and a toast says *Removed “…”.* with **Undo** (§2.4b).
  - The server is only told once the toast runs out, so Undo has nothing to put back; closing the tab before then keeps the event.
  - If the server refuses, the event comes back and a toast says *Couldn't remove “…”.* and why.
- **Motion:** events slide in when saved and out when removed, and the others glide into their new places.
- **Year handle (owner's pick, 2026-11-22):** when the timeline spans two years or more, a green handle sits on the line and stays in view while you scroll (42% down the window).
  - It shows the year it's on (*’84*; *—* for undated), between **▲** (newer) and **▼** (older), which glide there.
  - Dragging its middle scrubs a year per 26px; the up and down arrow keys step too.
  - The year labels it's on turn `--accent` and grow slightly; scrolling moves it along.
  - On phones the line moves in 21px to make room, and the year stops just above the handle instead of beside it.
- **+ New event, as built:** shown to admins only, since members can't add events yet.
  - **Motion (owner's pick, 2026-11-22):** the panel grows open and shrinks closed; the precision switch's white pill glides to the choice; Save shows three bouncing dots while saving, then a check drawing itself with *Saved* for 0.7s before the panel closes.
  - The date control follows the precision picked:
    - **Exact day:** the browser's date picker, as for birthdays.
    - **Month:** a month menu beside a year box. Safari has no month picker.
    - **Year** and **About:** a year box.
    - Save waits for a title, and for a four-digit year and a month when they're asked for. The date is optional.
  - **+ Add person** opens a name search of the tree (five matches at a time). Each pick becomes a chip with a remove ×.
  - Saving closes the panel, says *Saved "…"*, and the event takes its place on the timeline by date.
  - A refused save keeps the panel and what was typed, and says why.

### 4.6 Stories (own menu item)
- **Story of the week** hero: large video still with play button and duration, title, storyteller (avatar, relation, recorded date), short summary, people/topic chips, **Watch the story**.
- **Storytellers** row: filter by teller (shows count; older audio like "cassette, 1994" called out). Clicking again clears.
- **All stories** grid: thumbnail, length, Video/Audio badge, title, teller · year, "About …". Topic chips. Search covers what was said, not just titles. **+ Add a story**.
- **Your drafts** (only when you have some; admins see everyone's): a row above All stories with the same cards, a "Draft" chip and the step reached, each with **Continue editing**, which reopens Add a story at that step.

### 4.7 A story
- Player: dark frame, big play/pause, progress bar, time, **Captions** (on by default, large 20px on dark), Full screen.
- Title, *"Recorded October 2026 in Rose's kitchen · filmed by Maya Pluviose"*.
- **Transcript** with timestamps; clicking a line jumps there; the current chapter's lines are highlighted; search box.
- Side: **Told by** card · **Chapters** (click to jump; current one highlighted) · **People in this story** (can include people not in the tree) and matching years on the timeline · **More from Rose**.

### 4.7b Add a story (from **+ Add a story** on Stories; admins, or members with `MEMBERS_CAN_CONTRIBUTE`)
A five-step guide with a clickable step bar (Storyteller · Recording · Details · Transcript · Share), **Back / Save as draft / Continue** at the bottom of every step.
1. **Storyteller:** search the tree; pick a person card (radio). "Not in the tree yet? Add them first."
2. **Recording:** choose **Upload a file** (drop zone, any phone video, voice memo or digitized cassette; large files upload in parts and resume if the connection drops; progress shows "72% · 2.4 GB") or **Record now** (camera preview, big record button, timer, tips: quiet room, phone sideways, sit close). Beside it, **Questions to ask** prompts (*"How did you and Lucien meet?"*, *"What did you bring with you when you left?"* …).
3. **Details:** title, where it was recorded, when (Exact day · Month · Year · About), "Filmed by <you>", short summary, topic pills (Growing up, Moving & immigration, Weddings, Work, Recipes, Holidays), people in this story (chips + Add person).
4. **Transcript:** "Ready, please check it" + *Spoken in* (Haitian Creole · French · English · A mix); note that Creole needs a human read-through; each line is an editable box with its timestamp; **Show English translation** switch shows a translation under each line. **Chapters** panel: small player + **Start a chapter here (21:30)**, list of chapters.
5. **Share:** required consent checkbox *"Rose agreed to share this recording with the family."*; choose **Save as draft** (only you and admins) or **Publish to the family**. Publishing is disabled until consent is ticked (drafts don't need it).
Finish screen: *"Your story is on its way"* with steps ✓ Recording uploaded · … Preparing the video · … Making captions; **Back to Stories** / **Add another story**. The story shows as "Processing…" in the Stories list until ready.

### 4.8 My profile
**Since 2026-11-21 (owner's call):** the Requests card, the Sign-in card and Suggest a relative have left this page for the account panel (§2.1), the Requests page (§4.8a) and the Account page; the side column keeps **See me in the tree**. Their behavior below is unchanged.

Photo + story editor · name and birthday · contact rows (phone, email, address, WhatsApp, Instagram, Facebook, others) with switches: **Only me / Visible to family** for email and social accounts, **Only me / Family can ask** for phone, WhatsApp and address; a **"Requests for your details"** card at the top of the side column (*"Darline Monestime asked for your phone number." — Share / Not now*; sharing reveals it to that one person; declining is silent), and under it **Shared with**: one line per person you shared with (*"Darline Monestime · phone number"*) and a **Stop sharing** button that revokes it; add button reads *"+ Add a phone, email, address or social account"* · Sign-in (Change password, Sign out) · **"Know someone who should be here? Suggest a relative"** · **See me in the tree**.

Not drawn in the mockup, settled with the owner (November 2026):
- **Your story** has a **Save** button that stays under the box, dimmed until the text changes; **Cancel** appears beside it while there's an unsaved change. Saving says *"Saved."*
- **Name, birthday and photo:** a quiet **Edit** link under the birthday line, or a click on the initials circle (which carries a small camera badge), opens one form in place: First name, Last name, Birthday, with Save / Cancel. The circle stays on the left with *"Photos are coming soon."* until uploads arrive (Phase 5). People fix their own details, since admins can't know every birthday (owner's call). A partial date on file (*"1978"*) stays unless a full one is picked; emptying a full date removes it. The mockup's *"· only you and admins see the year"* is dropped (owner's call).
- **Contact rows:** a quiet **Edit** link beside each row's label opens an inline form (the value and an optional label such as "Mobile" or "Work"; type and platform never change), with **Remove** inside it. The add button opens the same form with a **Type** list first. New rows start as "Only me". Rows keep the mockup's order: phone, email, address, WhatsApp, then social accounts.
- **Change password** opens inside the Sign-in card: current password, new password with the strength meter, Save / Cancel. Afterwards: *"Password changed. You're signed out on your other devices."* The card's status line reads *"Password set"* (no Google).
- **Phone and WhatsApp numbers** get dashes once all 10 digits are in, with no +1 needed: *1115550000* reads *111-555-0000*, and *11115550000* reads *1-111-555-0000*. Any other number (with a **+**, or shorter or longer, like a Haitian *3412-3456*) stays as typed.
- The mockup's line *"Admins can see all information you enter."* is dropped (owner's call); the invitation email and First setup still say it.
- A login not yet linked to anyone in the tree sees *"Your login isn't linked to you in the tree yet, so there's no profile to fill in. An admin can link it."* and the Sign-in card.
- **Requests for your details** stays at the top of the side column even with nothing to answer: *"No requests right now."* **Share** and **Don't share** each ask first, in the confirm dialog (§2.4a; inside the request until 2026-11-22): *"Share your phone number with Darline?"* with *"Darline will see every phone number you've set to “Family can ask”, including ones you add later."* **Yes, share** / **Cancel**, or *"Don't share your phone number with Darline? You can't change your mind later."* **Yes, don't share** / **Cancel** (owner's calls: against an answer given by accident, and the mockup's **Not now** reads **Don't share**, since a decline is final). An answered request stays in place for the rest of the visit, saying *"Shared with Darline."* or *"Not shared. Darline won't be told."* **Shared with** sits inside the same card, under the requests, once something is shared. A share covers every detail of that kind set to "Family can ask", later ones too (architecture §5.2), so the card's footer says so instead of the mockup's *"Sharing shows it to that one person only."*: *"Sharing shows that one person every detail of that kind you've set to “Family can ask”, including ones you add later. You can stop sharing any time."*
- **Stop sharing** asks first, in the confirm dialog (§2.4a): *"Stop sharing your phone number with Darline?"* **Yes, stop** / **Keep sharing**. A share taken back can't be given again, and they can't ask twice; **Don't share** is just as final, only silent. Afterwards: *"Stopped sharing your phone number with Darline."*
- **Suggest a relative**: the card's line reads *"Suggest a relative and an admin will take a look. If they approve, your relative gets an invitation by email."* (the mockup's *"… an admin will send them an invitation"* promised too much). The button opens a form inside the card: **Their full name**, **Their email**, *"How are they related? (optional)"* as *"They're the [—] of [your name]"* (Request access's list; the name starts as yours and can be changed, so the admins' queue can find it in the tree), *"Anything else? (optional)"*, then **Send suggestion** / **Cancel**. Afterwards: *"Thanks. An admin will take a look, and if they approve, Darline gets an invitation by email."* and **Suggest someone else**. A login linked to nobody sees neither card.

### 4.8a Requests (`/requests`)
Owner's pick R1, 2026-11-21: requests for your details on their own page, opened from the account panel. Title *Requests*, then *"Family asking for your phone number, WhatsApp or address. They only see it if you say yes."* Three tabs: **Waiting** (with its count), **Answered** (*✓ Shared*, *Not shared* or *Stopped sharing*, newest first) and **Shared with** (Stop sharing, as before). The tabs are Radix Tabs (arrow keys, Home and End move and select; one Tab stop), with a 2.5px `--accent` underline that glides to the tab picked (2026-11-22). Share, Don't share and Stop sharing keep §4.8's confirm steps and wording; an answer given on this visit stays in Waiting, saying what happened. The server now lists answered requests too (pending first); a requester is still never told of a decline. The footer keeps the sharing note.

### 4.9 Full profile (from "Open full profile" / "View profile")
Back to the tree · portrait, relation, name, maiden name, birthday, place, story · **Center the tree on Nadège** and a dashed **Request phone number** button (never the number itself, unless she has shared it with you) · family grouped as Parents / Married to / Siblings / Children / Step-children / Grandchildren · photos · moments (events + stories).
- Foster parents and children are listed with their own (Parents, Children), marked *"Foster parent"* / *"Foster child"* when they have no relation to the viewer, so an admin sees a foster link they added.
- **Adding a relative (admins only):** no mockup in the design; it follows a prototype the owner approved on 2026-11-20. Parents, the last partner group (or an empty *"Partners"*), Siblings and Children each end in a dashed card: *"Add a parent"*, *"Add a partner"*, *"Add a brother or sister"*, *"Add a child"*. For admins these groups show even when empty. The card opens a bottom sheet on every screen (Radix Dialog; owner's pick, 2026-11-22): it springs up from the bottom edge, and dragging its grab handle down, or Escape, Cancel or a tap on the dimmed page, slides it away. It asks one question per screen, an answer moving on by itself, with Back, Cancel and step dots:
  - *"Who is Nadège's child?"*: one search box over the tree, its last row *"Add "Paolo Pluviose" as someone new"* (first word first name, the rest last name). Someone already related that way is listed but can't be picked (*"Already Nadège's child"*).
  - Parent or child: *"Birth mother / Adopted Nadège / Fostered Nadège"* or *"Born to / Adopted by / Fostered by Nadège"*. A child then asks *"Who is Paolo's other parent?"* among her partners, or *"Someone else, or not known yet"*.
  - Partner: *"Are they married?"*, then *"Are they still together?"*. Sibling: *"Which parents do they share?"* when more than one is recorded, *"Both"* offered only when there are exactly two; always a birth sibling (an adopted one is added as a child from the parent's profile). With no parents recorded, it asks for a parent first.
  - Last: *"Add Paolo Pluviose?"* with both people and the link word between them, a sentence, and one button. The new card is then highlighted, and a toast offers **Undo** (taking off the links, and the person if they were new) until it's put away with *"Done"* or the admin leaves the profiles; it has no time limit, and a retry after a failed Undo carries on where it stopped.

### 4.10 Members & invites (admins only)
Tabs: **Requests (n)** · **Invite someone** · **Everyone (n)**.
- **Requests:** name + email, a chip (*Pluviose family* or *Married into the family*; an older Monestime-side request shows as family), source chip ("Asked on the sign-in page" / "Suggested by Stanley Jean-Louis"), **Related to** lines with a badge **✓ In the tree** or **Not found — check spelling**, their note, when. **Approve & invite** → Invite tab with that person preselected. **Not now** → quietly set aside (they're never told), with Undo for 90 days (declined requests are deleted after that).
- **Invite someone:** 1) pick the person **in the tree first** ("Not in the tree yet? Add them first") · 2) email + role (Member / Admin, each with a one-line description) · 3) **Who is the invitation from?** (defaults to the member who suggested them, otherwise you; other admins selectable) · 4) live email preview. **Send invitation** → *"Invitation sent to Kervens, from Stanley Jean-Louis."*
- **Email copy:** subject *"<From first name> invited you to the family hub"*; body *"Bonjou Kervens, <From full name> invited you to the Pluviose Family Hub — a private place for our family's tree, photos and stories."*; if the sender isn't the admin who approved it, add *"Approved by <admin>, admin."*; button **Set up my account**; small print about the 7 days and admins seeing everything.
- **Everyone:** table (scrolls sideways on small screens): name, sign-in email (admins only), role, status (Active · Invited · expires in N days · Invited · expired · Disabled), and one action (**Send password reset** / **Resend invitation**).

Wording rule everywhere: say **"admin"**, not "family admin".

---

## 5. Product rules settled during design

1. Sending invitations is **admin-only**. Members can **suggest** a relative; suggestions land in Requests.
2. Admins build the tree ahead of time; an invitation is always **linked to an existing person**, so new members land in the right place.
3. The invitation shows who it's **from** (the suggesting member if there is one); the record also keeps who **sent** it.
4. Request access exists (this reverses plan §16.4), always answers the same way, and never creates an account.
5. No surnames in welcoming copy (inclusive of in-laws and name changes).
6. Contact details, social accounts included, default to "Only me". Phone numbers, WhatsApp and addresses are never shown in lists; they are shared person-to-person on request.

---

## 6. Trees (illustration and motion)

Everything here is decorative; with reduced motion, show the final frame without animation.

### 6.1 Sign-in tree (all five pre-login pages)
Generated once (seeded, deterministic) and embedded as inline SVG:
- **Shape:** trunk 235 units from the ground, recursive branching to 6 levels (2 splits, sometimes 3 near the trunk), spread 23°→19° per level, each branch 70–80% of its parent, slight random bend, angles pulled 22% toward vertical so the crown is taller than wide. Stroke widths 7.5 → 1.4; round caps. ~136 branches.
- **Leaves (~116):** at most forks, part-way along some branches, and every tip; radius 7–16; palette in §1.1.
- **Scene:** two faint mountain ridges (back peaks about halfway up the trunk, front ridge lower), a ground line, a few grass marks, 8 light roots below ground. **These never move.**
- **Growth timeline:** mountains draw 0.05–1.2s · ground 0.05s · roots 0.15–0.6s · trunk from 0.75s · every branch starts when its parent is 82% drawn, drawing at 195 units/s · leaves pop (0.4s scale-in) the moment their branch reaches them · fully grown ≈ 4.7s. **The app plays this whole timeline 2.35× faster (owner's choice, 2026-11-14), so the tree is fully grown in about 2s;** the leaf colour cycle and the sway keep their pace.
- **Drawing trick:** `pathLength="100"`, `stroke-dasharray: 100`, animate `stroke-dashoffset` 100→0, and keep **opacity 0 until the stroke starts**, otherwise round caps show as dots before the line draws.
- **After growth:** each leaf fades to a second neutral color and back on a **3s** cycle, starting 0.6s after it appears, neighbours offset by 0.3s (stand-in for photos switching). Only the trunk/branches/leaves group sways, ±1.5° over 4.5s, pivoting at the base of the trunk.

### 6.2 Home tree (small)
- Same size as the original hero illustration: **320 × 260**.
- Shape: trunk + **two main branches** (Dad, Mom) + upper trunk (You) + two short outer branches (Fritz, Sabine). Six branches, six small leaves.
- Same scene (two faint ridges, ground, grass, four roots), same rules: grows from the ground quickly (ground and roots by ~0.4s, trunk 0.42s, main branches 0.75s, top and outer 0.8–1.1s; fully grown ≈ 1.45s), each person pops when their branch arrives, only the tree sways (±1.5°, 6s, starting at 1.8s).
- People circles: You 22, parents 18, siblings 15 (radius). After growth they take turns showing their photo (cross-fade inside the circle + accent ring) every **3s** starting at 2s, on a 15s loop; the caption under the tree changes to match ("Mom · Christmas 1987"). Real photos come from media tagged to each person; fall back to initials.

### 6.3 Other motion
- Page content fades up 14px (0.6–0.8s).
- Tree page: generations cascade in (80ms apart, 0.4s each), lines draw in after them (0.4s), and the chain to the focus person fades green (150ms).
- Menu hover 150ms.
- Dialogs spring open (§2.4a). Motion's animations (the `motion` package) follow reduced motion and Pause motion through one `MotionConfig` at the app root.
- No looping motion anywhere else.

### 6.4 Pausing the motion
The swaying, the leaf colour cycle, and the Home photo/caption cycle run longer than 5 seconds beside other content, so each tree gets a **Pause motion / Play motion** text button (a real `<button>` whose text says what it will do, so no `aria-pressed`, 13px `--muted` under the tree or its caption, 44px tall hit area). Pausing freezes everything as it is (portraits stay on their initials) and stops the photo cycle; it's one choice for the whole site, remembered for the session. With reduced motion there is nothing to pause, so the button is hidden. (WCAG 2.2.2.) **Exception, owner's choice (2026-11-14):** the sign-in tree on the five pre-login pages has no pause button, as in their mockups. **Owner's choice (2026-11-21):** neither has the family tree page, for more room; its portraits follow the choice made on Home, and reduced motion still stops them. **Owner's choice (2026-11-22):** nor does the Home tree, whose leaves now sway; it follows reduced motion and a choice already made elsewhere. The Members book has none either, as drawn: its sheen, cover lift, peeking corner and portrait fades follow the same choice (§4.3).

---

## 7. Sample family used in the mockups (made up)

Signed-in viewer **Maya Pluviose** (admin). Parents Jean-Robert & Nadège Pluviose (née Monestime) · husband Stanley Jean-Louis · children Naïka & Mackenson Jean-Louis · granddaughter Naomie Désir (Naïka & Ralph Désir) · brother Fritz & Fabienne Pluviose (Kervens, Esther, Jonas) · half-sister Sabine Pluviose (Jean-Robert & Guerline Saint-Fleur) · Pluviose grandparents Joseph & Marie-Claire · great-grandparents Jean-Baptiste & Anne-Marie · Monestime grandparents Lucien & Rose · aunt Marie-Thérèse Pluviose (son Peterson) · uncle Frantz & aunt Mirlande Monestime (daughter Darline). Use these (or the seed family) for the Playwright smoke test and Storybook-style fixtures; never real relatives in the repo.
