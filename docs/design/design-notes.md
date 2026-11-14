# Family Hub: Frontend Design Notes ("Heirloom")

The agreed look and behaviour for the Pluviose-Monestime Family Hub frontend (Phase 4a/4b and later), from the design canvas of November 2026. The page files in `pages/` are the visual reference; this file is the spec to build from. Where the two disagree, this file wins.

All names, dates, stories, phone numbers and addresses in the mockups are **made up** sample data.

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

- Top: logo mark (52px circle, tree glyph) **centered above** "Pluviose-Monestime" (Fraunces 22px/600) and "FAMILY HUB" (13px small caps). The whole block links to Home.
- Items, in order: **Home · Family tree · People · Directory · Events · Stories · My profile**.
- **Admin** section (admins only), under a thin divider and small "ADMIN" label: **Members & invites**.
- Bottom: "Signed in as Maya".
- Current page: `--nav-active` background, 600 weight, `aria-current="page"`.
- **Hover / keyboard focus:** `--brand-panel` background plus `box-shadow: 0 1px 3px rgba(43,38,32,.12)`, transition 150ms. The current page keeps its own color and only gains the shadow.

### 2.2 Logo

Placeholder until a real logo exists: circle `#E6EEE9` with a `#2F5D50` outline, a small tree (trunk, two branches, top bud in green, two side leaves in `#8A6F4A`). Keep it simple and swappable.

### 2.3 Person cards

Initials avatar (circle, Fraunces), name (600), one line underneath with the relation **from the viewer's point of view** ("Your mother", "Your half-sister").

**No birthdays or dates in the family tree**: not on cards, chips or the quick look. Dates appear only on profiles and the birthday lists (People cards show none either, §4.3), following the masking rules (living people month/day only, deceased full years, "c. 1935" for approximate dates).

### 2.4 Card hover (everywhere)
Every card-style item lifts on hover **and** on keyboard focus inside it: `translate: 0 -4px` plus `box-shadow: 0 12px 26px rgba(43,38,32,.12)`, 200ms. Use the `translate` property (not `transform`) so it never fights a card's entrance animation. Applies to: Home cards and Story of the week, tree cards (which also scale slightly), People cards, Directory rows, Events timeline cards and Media tiles, Stories cards/storytellers/featured story, profile family cards, admin request cards. Off with reduced motion.

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
- "or" divider · **Continue with Google** (hidden when Google isn't configured).
- Small centered line: *"Already invited? Use the link in your invitation email."*
- No separate "New here?" box: the line under Sign in is the only Request access entry point.
- Never say "Welcome back" (we don't know who's visiting). Optional later: "Welcome back" only if this device has signed in before (a localStorage flag, no name stored).

### 3.2 Request access
- Intro: *"Tell us a little about yourself and an admin will take a look. If they approve, you'll get an invitation by email."* (No family surnames, since in-laws and married relatives may not carry them.)
- Full name · Email.
- **"How are you connected to the family?"**: Pluviose side · Monestime side · Married into the family. (No "family friend" option, on purpose.)
- **"How are you related?"** as one sentence: *I'm the [daughter ▾] of [name] and [name]*, with **+ Add another name** (up to 4). Relationship list: daughter, son, grandchild, sister, brother, niece or nephew, cousin, wife/husband/partner, mother or father, other relative. Names are **typed**, never suggested from the tree (that would reveal family names to strangers). Hint: *"Name a parent, grandparent or sibling if you can — it helps us find your place in the tree."*
- "Anything else? (optional)" textarea.
- **Send my request** → thank-you state: *"Your request is with the admins. If they approve it, you'll get an invitation email… usually within a few days."* + *"Know one of the admins? A quick call or text can speed things up."* Always the same response, whatever the email.

### 3.3 Welcome (from the invitation email)
- Banner with the inviter's avatar: *"**Nadège Pluviose** invited you to the family hub."* (Shows who the invitation is **from**, §5.3.)
- *"Welcome to the family, Darline"*; *"Choose a password to finish setting up your account for <email>."*
- **Continue with Google instead** (only for Gmail invitees: a Workspace address can't be recognised before sign-in, so those invitees use Continue with Google on the sign-in page) · "or choose a password" divider.
- New password with Show/Hide (shown by default here) and a 4-segment strength bar + label. No rules text up front; if the password is refused, show one plain-language error under the field (e.g. *"Use at least 8 characters"*, *"That password is too common — try a short phrase"*), linked with `aria-describedby`.
- **Set password and continue** → First-time setup.
- Small print: *"This link works for 7 days. Not Darline? Go to sign in."* (The "administrators can see everything" notice lives in the invitation email, First-time setup step 2 and My profile, not here.)

### 3.4 First-time setup (3 steps, progress bar, Back / Continue / Skip for now)
1. *"Let the family see who you are"*: photo (optional now) + "A line or two about you".
2. *"How can family reach you?"*: phone, email, address and **social accounts** (WhatsApp, Instagram, …), each with a switch, **all start as "Only me"**; label flips to "Visible to family" (email, social) or "Family can ask" (phone, WhatsApp, address). Footer: admins can see everything.
3. *"Here's your place in the family"*: grandparents → parents → **You** highlighted; *"Something look wrong? Let an admin know and they'll fix it."* → **Go to my home page**.

### 3.5 Forgot password (side path from Sign in)
- *"Forgot your password?"* + email + **Send me a reset link** → *"Check your email. If that email is on our family list, a reset link is on its way. It works for 30 minutes."* + spam/ask-an-admin hint. Google-only users can use it to add a password.

---

## 4. Signed-in pages

### 4.1 Home ("Hearth")
- Hero card: small caps day-part ("FRIDAY EVENING"), **"Good evening, Maya"** (52px), one personal line (*"Your mom's birthday is in 6 days, and Fritz added 12 photos from Thanksgiving."*), buttons **Open the family tree** / **See new photos**. Right side: the small family tree (§6.2) with a caption line under it.
- Three cards that rise in one after another (0.3s / 0.45s / 0.6s): **Coming up** (date tiles), **On this day** (two photos cross-fading every 5s + one line), **New in the family** (recent people, "See everyone"). On hover or keyboard focus inside, each card lifts 4px with a soft shadow (use the `translate` property, so it doesn't fight the rise-in animation's `transform`).
- **Story of the week** (full width, rises in at 0.75s, same hover lift): video still with play button and length on the left; on the right "STORY OF THE WEEK", title (Fraunces 32px), *"Told by Grandma Rose · recorded October 2026"*, a one-line quote from the transcript in Fraunces 19px, **Watch the story** / **More stories**. Driven by `stories.featured_at`; swappable later for "Photo of the week" or "Today in family history".

### 4.2 Family tree (focus view)
Built client-side from `/api/tree`.

Look ("tree watermark"):
- A **family strip** under the title: *38 people · 5 generations · Pluviose & Monestime lines · Earliest record: c. 1900, Jacmel* (pill chips).
- The tree sits on a `--surface-soft` panel (radius 22px). Behind everything, a **large, very pale tree** (`#EAE1D0` strokes, `#EFE7D8` leaf blobs) fills the panel: trunk rising from the bottom centre, limbs spreading behind each generation, a few roots. Purely decorative, never moves, `aria-hidden`.
- **Even proportions:** every person, grandparents and grandchildren included, uses **the same card**: 150 × 118, radius 14, a 50px framed portrait (cream + sand double ring) that fades between initials and a photo placeholder (6s, each person offset), name 15px/600, relation 13px. Same gap (14px) between cards in every row and the same spacing between generations.
- Each generation has a small uppercase label above it (*Grandparents · Your parents · Your household & siblings · Your children · Grandchildren*). Couples and parent groups sit in a soft rounded group (`rgba(255,253,248,.75)`, 1px border); your household uses `--accent-tint` with a 2px accent border.
- Grandparents / grandchildren with more family beyond them get a small **▲ more / ▼ more** pill under their card.
- No visible connector lines: the watermark tree does that job.

One **focus person**; shows:
- **Grandparents** (two generations up), grouped per parent, each card with **▲ more** when there's more family above.
- **Parents** household.
- **Focus row:** siblings split left and right · the focus person's **household** box (`--accent-tint`, 2px accent border) with current partners, and former partners with a dashed border labelled "Former partner".
- **Children**, then **grandchildren**, each grandchild card with **▼ more** when they have children.
- Half-siblings labelled from the viewer's point of view ("Your half-sister", or "… · half-sibling").

Interaction:
- **Hover or keyboard focus** on a card: the card lifts (`translateY(-3px) scale(1.04)`, soft shadow) and a **quick look** (250px card) appears just below it: relation, 2-line story, shared email or social account (never phone or address), **View profile ›**, and a quiet **Center here** link. (No dates.) No backdrop, nothing else covered.
- **Click** keeps the quick look open (that's how phones and tablets get it); click again to close. The click waits **230ms** before acting so a double-click can cancel it.
- **Double-click** centers the tree on that person (no quick-look flash).
- Centering animates: the tree slides **down** into place when moving to a descendant, **up** when moving to an ancestor, and fades/scales slightly sideways otherwise (0.5s).
- Trail above the tree: *Me › Naïka › Naomie* (last 8 steps; earlier steps clickable). **Back** (disabled at start) and **Back to me**.
- Search box suggests up to 6 matching names as you type; picking one centers the tree.
- Tip text: *"Hover or tap a name for a quick look · double-click to center the tree on them"*.
- An `aria-live` region announces "Family tree centered on …".
- Later: keyboard arrows and phone swipes to move a generation; URL per focus (`/tree/<person>`) so browser Back works and links can be shared.

### 4.3 People
Everyone in the archive (including those who have passed and people without accounts), **one card per person, A–Z by first name**, 5 across.
- Above the grid: title, search, the family strip (*38 people · 5 generations · …*), filter pills (Everyone · Living · Remembered · Pluviose side · Monestime side).
- **Card:** a 64px band across the top tinted by side (Pluviose `#EDF2EE`, Monestime `#F3ECE0`, both `#F1EDE3`, married in `#F2EFE8`; yours `--accent-tint`) with a faint mountain line; an 80px framed portrait overlapping the band (cream + sand ring) that fades between initials and a photo placeholder; name, relation, a side tag in the band color, and **"In memory"** with a small leaf for those who have passed. **No dates.** Lifts 3px with a soft shadow on hover/focus. Your card has an accent border.
- Each card links to the full profile.

### 4.4 Directory
The family phone book for **living relatives only**. One row per person (framed avatar, name, relation) with only what they chose to share:
- **Email**, shown with an **Email** button, if marked visible.
- **Social accounts** as small pill buttons with a neutral link icon and the platform name (Instagram, Facebook, TikTok, YouTube, X…), linking out. Use an official icon set for platform marks in the real build rather than hand-drawn logos.
- **Phone numbers, WhatsApp and addresses are never listed.** If the person allows it, a dashed **Request phone number / Request WhatsApp / Request address** button appears; after asking it becomes *"✓ Phone number requested"*. The person then decides in My profile (§4.8). WhatsApp counts as a phone number because opening a chat reveals it.
- Footer: only living members appear; phone/address are shared only on request; link to My profile.
People stays the page for *everyone* (including those who have passed and people without accounts); the two are kept separate on purpose.

### 4.5 Events
Title **Events**. Segmented **Timeline / Media**, plus two buttons shown to admins (and to members once `MEMBERS_CAN_CONTRIBUTE` is on):
- **+ New event** opens an inline panel: *What happened?*, *Where?*, *When?* with a precision switch (**Exact day · Month · Year · About**, matching the date + precision columns), *Tell the story* (optional), *Who was there?* (person chips + Add person), Cancel / **Save event**. Saved events appear at the top of the timeline.
- **+ Upload** opens an inline panel: drop zone (*"Drag photos and videos here"* + **Choose files**), the cap in plain words (*"Photos and videos up to 2 GB each. Recording a longer interview? Add it as a story instead."*), **Add to event** picker (or *No event — just add to Media*), then a progress row per file (thumbnail tile, name, bar, "64% · 1.1 of 1.7 GB").
- **Timeline:** year column, a dot on a vertical line, event cards with title, date (precision-aware: "c. 1962"), story, photo strip, tagged-people chips.
- **Media:** 5-column grid of photos **and videos**; videos show a play button and their length. Captions under each.

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
Photo + story editor · birthday shown with *"only you and admins see the year"* · contact rows (phone, email, address, WhatsApp, Instagram, Facebook, others) with switches: **Only me / Visible to family** for email and social accounts, **Only me / Family can ask** for phone, WhatsApp and address; a **"Requests for your details"** card at the top of the side column (*"Darline Monestime asked for your phone number." — Share / Not now*; sharing reveals it to that one person; declining is silent), and under it **Shared with**: one line per person you shared with (*"Darline Monestime · phone number"*) and a **Stop sharing** button that revokes it; add button reads *"+ Add a phone, email, address or social account"* · one quiet line under the contact list: *"Admins can see all information you enter."* · Sign-in (password set / Google linked, Change password, Sign out) · **"Know someone who should be here? Suggest a relative"** · **See me in the tree**.

### 4.9 Full profile (from "Open full profile" / "View profile")
Back to the tree · portrait, relation, name, maiden name, birthday, place, story · **Center the tree on Nadège** and a dashed **Request phone number** button (never the number itself, unless she has shared it with you) · family grouped as Parents / Married to / Children / Step-children / Grandchildren · photos · moments (events + stories).

### 4.10 Members & invites (admins only)
Tabs: **Requests (n)** · **Invite someone** · **Everyone (n)**.
- **Requests:** name + email, side chip, source chip ("Asked on the sign-in page" / "Suggested by Stanley Jean-Louis"), **Related to** lines with a badge **✓ In the tree** or **Not found — check spelling**, their note, when. **Approve & invite** → Invite tab with that person preselected. **Not now** → quietly set aside (they're never told), with Undo for 90 days (declined requests are deleted after that).
- **Invite someone:** 1) pick the person **in the tree first** ("Not in the tree yet? Add them first") · 2) email + role (Member / Admin, each with a one-line description) · 3) **Who is the invitation from?** (defaults to the member who suggested them, otherwise you; other admins selectable) · 4) live email preview. **Send invitation** → *"Invitation sent to Kervens, from Stanley Jean-Louis."*
- **Email copy:** subject *"<From first name> invited you to the family hub"*; body *"Bonjou Kervens, <From full name> invited you to the Pluviose-Monestime Family Hub — a private place for our family's tree, photos and stories."*; if the sender isn't the admin who approved it, add *"Approved by <admin>, admin."*; button **Set up my account**; small print about Google, 7 days, admins seeing everything.
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
- Tree-page re-centering slide (0.5s) and quick-look pop (0.16s).
- Menu hover 150ms.
- No looping motion anywhere else.

### 6.4 Pausing the motion
The swaying, the leaf colour cycle, the Home photo/caption cycle and the People portrait fade run longer than 5 seconds beside other content, so each tree and the People grid get a **Pause motion / Play motion** text button (a real `<button>` with `aria-pressed`, 13px `--muted` under the tree or its caption, and on People at the end of the filter row, 44px tall hit area). Pausing freezes everything as it is (portraits stay on their initials) and stops the photo cycle; it's one choice for the whole site, remembered for the session. With reduced motion there is nothing to pause, so the button is hidden. (WCAG 2.2.2.) **Exception, owner's choice (2026-11-14):** the sign-in tree on the five pre-login pages has no pause button, as in their mockups.

---

## 7. Sample family used in the mockups (made up)

Signed-in viewer **Maya Pluviose** (admin). Parents Jean-Robert & Nadège Pluviose (née Monestime) · husband Stanley Jean-Louis · children Naïka & Mackenson Jean-Louis · granddaughter Naomie Désir (Naïka & Ralph Désir) · brother Fritz & Fabienne Pluviose (Kervens, Esther, Jonas) · half-sister Sabine Pluviose (Jean-Robert & Guerline Saint-Fleur) · Pluviose grandparents Joseph & Marie-Claire · great-grandparents Jean-Baptiste & Anne-Marie · Monestime grandparents Lucien & Rose · aunt Marie-Thérèse Pluviose (son Peterson) · uncle Frantz & aunt Mirlande Monestime (daughter Darline). Use these (or the seed family) for the Playwright smoke test and Storybook-style fixtures; never real relatives in the repo.
