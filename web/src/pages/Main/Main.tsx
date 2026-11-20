import { useQuery } from '@tanstack/react-query'
import { type ReactNode, type Ref, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'

import { type Contact, type ContactMethod, contactsQuery, isRequestable, kindOf, platformLabel } from '../../api/contacts'
import { familySummaryQuery } from '../../api/family'
import { type TreePerson, treeQuery } from '../../api/tree'
import { FamilyStrip } from '../../components/FamilyStrip/FamilyStrip'
import { PhotoFade } from '../../components/PhotoFade/PhotoFade'
import { errorMessage } from '../../lib/errorMessage'
import { familyLinks, type FamilyLinks } from '../../lib/familyLinks'
import { focusView, type FocusView, treeChildrenOf, treeParentsOf } from '../../lib/familyView'
import { useMotionPaused, useReducedMotion } from '../../lib/motion'
import { fullName, initials, nameMatches } from '../../lib/names'
import { relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './Main.module.css'

const trailLength = 8
const maxMatches = 6
// A click waits this long so a double-click can cancel it (design-notes §4.2).
const clickDelay = 230

type Direction = 'up' | 'down' | 'side'

// Family tree (design-notes §4.2): one focus person with the generations around them, built from /api/tree.
export function Main() {
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const tree = useQuery(treeQuery)
  const summary = useQuery(familySummaryQuery)
  const family = useMemo(() => tree.data && familyLinks(tree.data), [tree.data])
  const people = useMemo(() => new Map(tree.data?.persons.map((p) => [p.id, p])), [tree.data])
  const relations = useMemo(
    () => (tree.data && family && viewerId ? relationsTo(tree.data, viewerId, family) : new Map<string, string>()),
    [tree.data, family, viewerId],
  )

  // Until someone is picked, the trail is just you; people since removed from the tree drop out of it.
  const [picked, setPicked] = useState<string[] | null>(null)
  const trail = (picked ?? (viewerId ? [viewerId] : []))
    .filter((id) => people.has(id))
    .filter((id, i, kept) => id !== kept[i - 1])
  const focus = trail.at(-1) ?? null
  const view = useMemo(() => (family && focus ? focusView(family, focus) : null), [family, focus])
  const [direction, setDirection] = useState<Direction | null>(null)
  const [status, setStatus] = useState('')

  const moveTo = (next: string[]) => {
    const to = next.at(-1)
    if (to && focus && to !== focus && view) {
      const above = view.parents.includes(to) || view.grandparents.some((g) => g.ids.includes(to))
      const below = view.children.some((g) => g.ids.includes(to)) || view.grandchildren.some((g) => g.ids.includes(to))
      setDirection(above ? 'up' : below ? 'down' : 'side')
      setStatus(`Family tree centered on ${to === viewerId ? 'you' : fullName(people.get(to)!)}`)
    } else if (to && !focus) {
      setStatus(`Family tree centered on ${fullName(people.get(to)!)}`)
    }
    setPicked(next)
  }
  const centerOn = (id: string) => {
    if (id !== focus) {
      moveTo([...trail, id].slice(-trailLength))
    }
  }

  const [paused, setPaused] = useMotionPaused()
  const reducedMotion = useReducedMotion()

  return (
    <main className={styles.page}>
      <title>Family tree — Pluviose-Monestime Family Hub</title>
      <div className={styles.top}>
        <h1 className={styles.title}>Family tree</h1>
        <Search persons={tree.data?.persons ?? []} relations={relations} onPick={centerOn} />
        <button
          type="button"
          className={`pill ${styles.back}`}
          disabled={trail.length < 2}
          onClick={() => moveTo(trail.slice(0, -1))}
        >
          Back
        </button>
        {viewerId && people.has(viewerId) && (
          <button type="button" className={`pill ${styles.home}`} onClick={() => centerOn(viewerId)}>
            Back to me
          </button>
        )}
      </div>

      <nav aria-label="Where you've been" className={styles.trail}>
        {trail.map((id, i) => (
          <span key={`${i}-${id}`} className={styles.step}>
            {i > 0 && <span aria-hidden="true">›</span>}
            {i === trail.length - 1 ? (
              <span aria-current="true" className={styles.here}>
                {id === viewerId ? 'Me' : people.get(id)!.first_name}
              </span>
            ) : (
              <button type="button" className={styles.stepButton} onClick={() => moveTo(trail.slice(0, i + 1))}>
                {id === viewerId ? 'Me' : people.get(id)!.first_name}
              </button>
            )}
          </span>
        ))}
        <span className={styles.tip}>Hover or tap a name for a quick look · double-click to center the tree on them</span>
      </nav>

      {summary.data && <FamilyStrip summary={summary.data} />}
      <p aria-live="polite" className="visually-hidden">
        {status}
      </p>

      {tree.error && !tree.data ? (
        <p role="alert">{errorMessage(tree.error)}</p>
      ) : !tree.data || !family ? (
        <p className={styles.note}>Loading…</p>
      ) : tree.data.persons.length === 0 ? (
        <p className={styles.note}>Nobody is in the family tree yet.</p>
      ) : !focus || !view ? (
        <p className={styles.note}>Your login isn’t linked to anyone in the tree yet. Find a relative above to start.</p>
      ) : (
        <FocusTree
          key={focus}
          people={people}
          family={family}
          view={view}
          focus={focus}
          viewerId={viewerId}
          relations={relations}
          direction={direction}
          paused={paused}
          onCenter={centerOn}
        />
      )}

      {/* The portraits' fade runs on; with reduced motion there's none to pause (design-notes §6.4). */}
      {!reducedMotion && focus && (
        <button type="button" className={styles.pause} onClick={() => setPaused(!paused)}>
          {paused ? 'Play motion' : 'Pause motion'}
        </button>
      )}
    </main>
  )
}

function Search({
  persons,
  relations,
  onPick,
}: {
  persons: TreePerson[]
  relations: Map<string, string>
  onPick: (id: string) => void
}) {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const matches = open && query.trim() ? persons.filter((p) => nameMatches(fullName(p), query)).slice(0, maxMatches) : []
  return (
    <div
      className={styles.search}
      onFocus={() => setOpen(true)}
      onBlur={(event) => !event.currentTarget.contains(event.relatedTarget) && setOpen(false)}
    >
      <label htmlFor="tree-search" className="visually-hidden">
        Find a relative
      </label>
      <input
        id="tree-search"
        type="search"
        autoComplete="off"
        placeholder="Find a relative…"
        value={query}
        onChange={(event) => {
          setQuery(event.target.value)
          setOpen(true)
        }}
        onKeyDown={(event) => event.key === 'Escape' && setQuery('')}
        className={styles.searchInput}
      />
      {matches.length > 0 && (
        <ul aria-label="Matching relatives" className={styles.matches}>
          {matches.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                className={styles.match}
                // Keeps focus in the box: Safari doesn't focus a clicked button, so the list would close first.
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => {
                  setQuery('')
                  onPick(p.id)
                }}
              >
                <span className={styles.matchName}>{fullName(p)}</span>
                {relations.get(p.id) && <span className={styles.matchRelation}>{relations.get(p.id)}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

// A small label on adoptive and foster links (architecture §7), said from each side, with a whole label
// when the viewer has no relation to them. Centered on you, the relation already says foster ("Foster mother").
const parentWords = {
  biological: null,
  adoptive: { word: 'adoptive', alone: 'Adoptive parent' },
  foster: { word: 'foster', alone: 'Foster parent' },
} as const
const childWords = {
  biological: null,
  adoptive: { word: 'adopted', alone: 'Adopted child' },
  foster: { word: 'foster', alone: 'Foster child' },
} as const

function FocusTree({
  people,
  family,
  view,
  focus,
  viewerId,
  relations,
  direction,
  paused,
  onCenter,
}: {
  people: Map<string, TreePerson>
  family: FamilyLinks
  view: FocusView
  focus: string
  viewerId: string | null
  relations: Map<string, string>
  direction: Direction | null
  paused: boolean
  onCenter: (id: string) => void
}) {
  const first = (id: string) => people.get(id)!.first_name
  const you = focus === viewerId
  const whose = (words: string) => (you ? `Your ${words}` : `${first(focus)}’s ${words}`)

  // What the quick look shows: hovered or keyboard-focused, else the one a click kept open.
  const [hover, setHover] = useState<string | null>(null)
  const [pinned, setPinned] = useState<string | null>(null)
  const open = hover ?? pinned
  const clickTimer = useRef(0)
  useEffect(() => () => window.clearTimeout(clickTimer.current), [])
  useEffect(() => {
    const close = (event: PointerEvent) => {
      if (!(event.target as Element).closest?.('[data-person]')) {
        setPinned(null)
      }
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setPinned(null)
        setHover(null)
      }
    }
    document.addEventListener('pointerdown', close)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('pointerdown', close)
      document.removeEventListener('keydown', escape)
    }
  }, [])

  // When the pressed button is gone with the old view, keyboard focus lands on the new focus card,
  // quietly: that focus shouldn't open its quick look.
  const focusCard = useRef<HTMLButtonElement>(null)
  const quietFocus = useRef(false)
  useEffect(() => {
    if (direction && (!document.activeElement || document.activeElement === document.body)) {
      quietFocus.current = true
      focusCard.current?.focus({ preventScroll: true })
      quietFocus.current = false
    }
  }, [direction])

  const card = (id: string, { sub, former, more }: { sub?: string; former?: boolean; more?: 'up' | 'down' } = {}) => (
    <Person
      key={id}
      person={people.get(id)!}
      sub={sub ?? relations.get(id) ?? ''}
      relation={relations.get(id) ?? null}
      isFocus={id === focus}
      former={former}
      paused={paused}
      more={more}
      open={open === id}
      buttonRef={id === focus ? focusCard : undefined}
      onHover={(on) => !quietFocus.current && setHover((h) => (on ? id : h === id ? null : h))}
      onClick={() => {
        window.clearTimeout(clickTimer.current)
        const closing = pinned === id
        clickTimer.current = window.setTimeout(() => {
          setPinned(closing ? null : id)
          // A second tap closes it, even while the mouse still rests on the card.
          if (closing) {
            setHover(null)
          }
        }, clickDelay)
      }}
      onDoubleClick={() => {
        window.clearTimeout(clickTimer.current)
        onCenter(id)
      }}
      onCenter={() => onCenter(id)}
    />
  )
  const labelled = (id: string, link: { word: string; alone: string } | null) => {
    const rel = relations.get(id) ?? ''
    if (!link || (you && link.word === 'foster')) {
      return rel
    }
    return rel ? `${rel} · ${link.word}` : link.alone
  }

  const siblings = view.siblings.map(({ id, half, foster }) => {
    const rel = relations.get(id) ?? ''
    // From another focus, the viewer's label may not say half or foster: say it from the focus person's side.
    const word = you || id === viewerId ? null : foster ? 'foster sibling' : half && !(viewerId && family.halfSiblings(viewerId, id)) ? 'half-sibling' : null
    return card(id, { sub: word ? (rel ? `${rel} · ${word}` : word[0]!.toUpperCase() + word.slice(1)) : rel })
  })
  const split = Math.ceil(siblings.length / 2)

  const rows = useRef<HTMLDivElement>(null)
  const panel = useRef<HTMLElement>(null)
  const [lines, setLines] = useState<{ width: number; height: number; paths: Line[] }>({ width: 0, height: 0, paths: [] })
  useLayoutEffect(() => {
    const p = panel.current
    const r = rows.current
    if (!p || !r) {
      return
    }
    const draw = () => setLines({ width: p.clientWidth, height: p.clientHeight, paths: measure(p, r, view, focus) })
    draw()
    // Cards move without the panel resizing when the web fonts land or a row rewraps.
    const observer = new ResizeObserver(draw)
    observer.observe(p)
    observer.observe(r)
    let live = true
    void document.fonts?.ready.then(() => live && draw())
    return () => {
      live = false
      observer.disconnect()
    }
  }, [view, focus])

  return (
    <section ref={panel} aria-label={`Family tree centered on ${you ? 'you' : fullName(people.get(focus)!)}`} className={styles.panel}>
      <div aria-hidden="true" className={styles.lines}>
        <svg width={lines.width} height={lines.height}>
          {lines.paths.map((l) => (
            <path key={l.key} d={l.d} />
          ))}
        </svg>
      </div>
      <div ref={rows} className={direction ? `${styles.rows} ${styles[direction]}` : styles.rows}>
        {view.grandparents.length > 0 && (
          <Generation label="Grandparents">
            {view.grandparents.map((g) => (
              <Group key={g.of} unit={`up-${g.of}`} label={`${first(g.of)}’s parents`}>
                {g.ids.map((id) => card(id, { more: treeParentsOf(family, id).length > 0 ? 'up' : undefined }))}
              </Group>
            ))}
          </Generation>
        )}
        {view.parents.length > 0 && (
          <Generation label={whose('parents')}>
            <Group unit="parents">{view.parents.map((id) => card(id, { sub: labelled(id, parentWords[family.linkOf(focus, id) ?? 'biological']) }))}</Group>
          </Generation>
        )}
        <Generation label={whose('household & siblings')}>
          {siblings.slice(0, split)}
          <Group unit="household" household>
            {card(focus)}
            {view.partners.map((id) => card(id))}
            {/* The dashed outline marks a former partner; the words stay the viewer's relation where there is one. */}
            {view.formerPartners.map((id) => card(id, { former: true, sub: relations.get(id) ?? 'Former partner' }))}
          </Group>
          {siblings.slice(split)}
        </Generation>
        {view.children.length > 0 && (
          <Generation label={whose('children')}>
            {view.children.map((g, i) => (
              <Group
                key={g.with.join(' ') || 'none'}
                unit={`children-${i}`}
                label={g.with.length > 0 && (view.children.length > 1 || view.partners.length + view.formerPartners.length > 1) ? `With ${g.with.map(first).join(' & ')}` : undefined}
              >
                {g.ids.map((id) => card(id, { sub: labelled(id, childWords[family.linkOf(id, focus) ?? 'biological']) }))}
              </Group>
            ))}
          </Generation>
        )}
        {view.grandchildren.length > 0 && (
          <Generation label="Grandchildren">
            {view.grandchildren.map((g) => (
              <Group key={g.of} unit={`down-${g.of}`} label={`${first(g.of)}’s children`}>
                {g.ids.map((id) => card(id, { more: treeChildrenOf(family, id).length > 0 ? 'down' : undefined }))}
              </Group>
            ))}
          </Generation>
        )}
      </div>
    </section>
  )
}

function Generation({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className={styles.generation}>
      <span className={styles.generationLabel}>{label}</span>
      <div className={styles.row}>{children}</div>
    </div>
  )
}

function Group({ unit, label, household, children }: { unit: string; label?: string; household?: boolean; children: ReactNode }) {
  return (
    <div data-unit={unit} className={household ? `${styles.group} ${styles.household}` : styles.group}>
      {label && <span className={styles.groupLabel}>{label}</span>}
      <div className={styles.groupRow}>{children}</div>
    </div>
  )
}

function Person({
  person,
  sub,
  relation,
  isFocus,
  former,
  paused,
  more,
  open,
  buttonRef,
  onHover,
  onClick,
  onDoubleClick,
  onCenter,
}: {
  person: TreePerson
  sub: string
  relation: string | null
  isFocus: boolean
  former?: boolean
  paused: boolean
  more?: 'up' | 'down'
  open: boolean
  buttonRef?: Ref<HTMLButtonElement>
  onHover: (on: boolean) => void
  onClick: () => void
  onDoubleClick: () => void
  onCenter: () => void
}) {
  const classes = [styles.card, isFocus && styles.focusCard, former && styles.former, open && styles.lifted].filter(Boolean).join(' ')
  return (
    <div
      data-person={person.id}
      className={styles.person}
      onPointerEnter={(event) => event.pointerType === 'mouse' && onHover(true)}
      onPointerLeave={(event) => event.pointerType === 'mouse' && onHover(false)}
      onFocus={() => onHover(true)}
      onBlur={(event) => !event.currentTarget.contains(event.relatedTarget) && onHover(false)}
    >
      <button
        ref={buttonRef}
        type="button"
        aria-expanded={open}
        className={classes}
        onClick={onClick}
        onDoubleClick={onDoubleClick}
      >
        <span aria-hidden="true" className={isFocus ? `${styles.avatar} ${styles.focusAvatar}` : styles.avatar}>
          {initials(person)}
          <PhotoFade person={person} paused={paused} />
        </span>
        <span className={styles.name}>{fullName(person)}</span>
        {sub && <span className={styles.sub}>{sub}</span>}
      </button>
      {more && (
        <button type="button" className={`pill ${styles.more}`} aria-label={`More: center the tree on ${fullName(person)}`} onClick={onCenter}>
          {more === 'up' ? '▲ more' : '▼ more'}
        </button>
      )}
      {open && <Glance person={person} relation={relation} isFocus={isFocus} onCenter={onCenter} />}
    </div>
  )
}

// The quick look under a card: relation, two lines of story, one shared email or social account, and links.
function Glance({ person, relation, isFocus, onCenter }: { person: TreePerson; relation: string | null; isFocus: boolean; onCenter: () => void }) {
  const box = useRef<HTMLDivElement>(null)
  const [shift, setShift] = useState(0)
  // Kept inside the tree panel, so a card at the edge doesn't push the page sideways. Offsets, like the
  // lines, so a slide-in under way doesn't skew it.
  useLayoutEffect(() => {
    const el = box.current
    const anchor = el?.parentElement
    const panel = el?.closest('section')
    if (!el || !anchor || !panel) {
      return
    }
    const place = () => {
      const half = el.offsetWidth / 2
      const { x } = offsetBox(anchor, panel)
      setShift(Math.max(8 - (x - half), Math.min(0, panel.clientWidth - 8 - (x + half))))
    }
    place()
    const observer = new ResizeObserver(place)
    observer.observe(panel)
    return () => observer.disconnect()
  }, [])
  return (
    <div ref={box} className={styles.glance} style={{ translate: `calc(-50% + ${shift}px) 0` }}>
      <div className={styles.glanceCard}>
        {relation && <span className={styles.glanceRelation}>{relation}</span>}
        {person.bio_excerpt && <span className={styles.glanceStory}>{person.bio_excerpt}</span>}
        <SharedContact personId={person.id} />
        <div className={styles.glanceFoot}>
          <Link to={`/people/${person.id}`} className={styles.profileLink}>
            View profile ›
          </Link>
          {!isFocus && (
            <button type="button" className={styles.centerHere} onClick={onCenter}>
              Center here
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

// Asked for afresh each time a quick look stays open a moment, never from a cache (architecture §7), so a
// contact just hidden is gone at once. Never a phone, WhatsApp or an address, even one shared with you.
function SharedContact({ personId }: { personId: string }) {
  const [settled, setSettled] = useState(false)
  useEffect(() => {
    const timer = window.setTimeout(() => setSettled(true), 150)
    return () => window.clearTimeout(timer)
  }, [])
  const contacts = useQuery({ ...contactsQuery(personId), enabled: settled, staleTime: 0 })
  // Admins and the person themselves get every row back, hidden ones too: show only what the family sees.
  const shared = contacts.data?.find(
    (c: Contact): c is ContactMethod =>
      !isRequestable(c) && (c.kind === 'email' || c.kind === 'social') && kindOf(c) === null && c.visibility === 'family',
  )
  if (!shared) {
    return null
  }
  return (
    <span>
      {shared.kind === 'email' ? 'Email' : platformLabel(shared.platform)} · {shared.value}
    </span>
  )
}

type Line = { key: string; d: string }

// Where an element rests inside the panel: offsets, not screen boxes, so the slide-in doesn't skew it.
function offsetBox(el: HTMLElement, panel: HTMLElement) {
  let x = 0
  let y = 0
  for (let n: HTMLElement | null = el; n && n !== panel; n = n.offsetParent as HTMLElement | null) {
    x += n.offsetLeft
    y += n.offsetTop
  }
  return { x: x + el.offsetWidth / 2, top: y, bottom: y + el.offsetHeight }
}

// Lines from each set of parents down to their children (owner's call, replacing the mockup's watermark tree).
function measure(panel: HTMLElement, rows: HTMLElement, view: FocusView, focus: string): Line[] {
  const box = (el: HTMLElement) => offsetBox(el, panel)
  const unit = (name: string) => rows.querySelector<HTMLElement>(`[data-unit="${name}"]`)
  const card = (id: string) => rows.querySelector<HTMLElement>(`[data-person="${id}"] button`)
  // A line into a group stops at the group's edge: you inside your household, children inside theirs.
  const links: { key: string; from: HTMLElement | null; to: HTMLElement | null }[] = [
    ...view.grandparents.map((g) => ({ key: `up:${g.of}`, from: unit(`up-${g.of}`), to: card(g.of) })),
    ...(view.parents.length
      ? [
          { key: `sib:${focus}`, from: unit('parents'), to: unit('household') },
          ...view.siblings.map((s) => ({ key: `sib:${s.id}`, from: unit('parents'), to: card(s.id) })),
        ]
      : []),
    ...view.children.map((_, i) => ({ key: `kids:${i}`, from: unit('household'), to: unit(`children-${i}`) })),
    ...view.grandchildren.map((g) => ({ key: `down:${g.of}`, from: card(g.of), to: unit(`down-${g.of}`) })),
  ]
  const drawn = links.filter((l): l is { key: string; from: HTMLElement; to: HTMLElement } => !!l.from && !!l.to)
  // The joining line runs just above the cards it feeds, so lines from the same parents share it; on a row
  // that wraps, the second line of cards gets its own, in the gap between the two.
  return drawn.map((l) => {
    const f = box(l.from)
    const t = box(l.to)
    const m = t.top - 12
    const dx = t.x - f.x
    if (Math.abs(dx) < 1) {
      return { key: l.key, d: `M${f.x} ${f.bottom} V${t.top}` }
    }
    const r = Math.max(0, Math.min(10, Math.abs(dx) / 2, m - f.bottom, t.top - m))
    const s = Math.sign(dx)
    return {
      key: l.key,
      d: `M${f.x} ${f.bottom} V${m - r} Q${f.x} ${m} ${f.x + s * r} ${m} H${t.x - s * r} Q${t.x} ${m} ${t.x} ${m + r} V${t.top}`,
    }
  })
}
