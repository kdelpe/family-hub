import { useQuery } from '@tanstack/react-query'
import { type CSSProperties, type PointerEvent as ReactPointerEvent, type ReactNode, type Ref, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'

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

const maxMatches = 6
const minZoom = 0.5
const maxZoom = 1.5
const clampZoom = (zoom: number) => Math.min(maxZoom, Math.max(minZoom, zoom))
// The buttons move in steps of 10%, landing on whole steps after a pinch.
const zoomStep = (zoom: number, by: number) => clampZoom(Math.round((zoom + by) * 10) / 10)
// Each generation arrives this many ms after the one above and has landed after rowLanded (handed to
// Main.module.css as --row-landed); a line draws once the generation it reaches has landed (design-notes §4.2).
const rowStagger = 80
// Recenter and the zoom reset glide the tree back this long (Main.module.css), rather than jump.
const glideMs = 300
// How far an arrow key moves the tree.
const keyStep = 60
// On a window too short or narrow for the page to fit, it scrolls: then a plain wheel and an up-and-down swipe are left
// to the page, as the way back to the controls above the tree. The same query as Main.module.css's, so the gestures
// and touch-action always agree.
const scrollingPage = '(max-width: 800px), (max-height: 519.98px)'
const pageScrolls = () => window.matchMedia?.(scrollingPage).matches ?? false
// The room kept around the focus person's card when the tree has to move to show it.
const cardRoom = 16
const rowLanded = 400
// Centering on someone, the cards still in view fly from where they were (design-notes §4.2), landing after flyMs; the
// lines draw in once they have.
const flyMs = 520
const flyEasing = 'cubic-bezier(0.3, 1.2, 0.45, 1)'
// The green runs along one line of the chain after another, from the lifted card to the focus person.
const chainStep = 180
// A flung tree slows by this much each frame (of 16ms), and is pulled back this hard when past its limits; dragged
// past them, it moves only this part of the way.
const flingFriction = 0.92
const flingPull = 0.18
const stretch = 0.35
// The endpaper's dots, as in the Members book: they move and scale with the tree, as if it were drawn on them.
const dotGrid = 30
// A line draws in this long (Main.module.css's draw).
const lineDraw = 400
// When a lift began and the lines started drawing, on one clock, to time the green against them.
const clock = () => performance.now()
const rowsStyle = { '--row-landed': `${rowLanded}ms` } as CSSProperties

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

  // Until someone is picked, the tree centers where it opened: on the person a profile's "Center the tree on …"
  // asked for, else you. Someone since removed from the tree is dropped.
  const [params] = useSearchParams()
  const requested = params.get('person')
  const start = requested && people.has(requested) ? requested : viewerId
  const [picked, setPicked] = useState<string | null>(null)
  const focus = [picked, start].find((id) => id && people.has(id)) ?? null
  const view = useMemo(() => (family && focus ? focusView(family, focus) : null), [family, focus])
  // Set once anyone is centered on, so keyboard focus can land on the new focus card; not on opening the page.
  const [centered, setCentered] = useState(false)
  const [status, setStatus] = useState('')
  // Counts centerings on the person already centered, which bring a moved tree back to the middle too.
  const [recentered, setRecentered] = useState(0)

  // Where each card was on screen before centering on `to`, for the new view's cards to fly from.
  const [flyFrom, setFlyFrom] = useState<{ to: string; rects: Map<string, DOMRect> } | null>(null)

  const centerOn = (to: string) => {
    setLine(null)
    if (to === focus) {
      setRecentered((n) => n + 1)
      return
    }
    setFlyFrom({ to, rects: cardRects() })
    setCentered(true)
    setStatus(`Family tree centered on ${to === viewerId ? 'you' : fullName(people.get(to)!)}`)
    setPicked(to)
  }
  // The people walked through with the ▲ / ▼ arrows, top to bottom, so the other arrow walks back along them;
  // dropped once any of them has left the tree.
  const [line, setLine] = useState<{ ids: string[]; at: number } | null>(null)
  const walked = line && line.ids[line.at] === focus && line.ids.every((id) => people.has(id)) ? line : focus ? { ids: [focus], at: 0 } : null
  const step = (by: 'up' | 'down', to: string) => {
    centerOn(to)
    if (!walked) {
      return
    }
    const { ids, at } = walked
    if (by === 'up') {
      setLine(ids[at - 1] === to ? { ids, at: at - 1 } : { ids: [to, ...ids.slice(at)], at: 0 })
    } else {
      setLine(ids[at + 1] === to ? { ids, at: at + 1 } : { ids: [...ids.slice(0, at + 1), to], at: at + 1 })
    }
  }

  // Paused from Home, which keeps the button (design-notes §6.4).
  const [paused] = useMotionPaused()
  const [zoom, setZoom] = useState(1)

  return (
    <main className={styles.page}>
      <title>Family tree — Pluviose Family Hub</title>
      <h1 className="visually-hidden">Family tree</h1>
      <div className={styles.top}>
        <div className={styles.stats}>{summary.data && <FamilyStrip summary={summary.data} />}</div>
        <div className={styles.controls}>
          <Search persons={tree.data?.persons ?? []} relations={relations} onPick={centerOn} />
          {viewerId && people.has(viewerId) && (
            <button type="button" className={`pill ${styles.home}`} onClick={() => centerOn(viewerId)}>
              Back to me
            </button>
          )}
        </div>
      </div>

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
          centered={centered}
          paused={paused}
          flyFrom={flyFrom}
          zoom={zoom}
          recentered={recentered}
          backUp={walked?.ids[walked.at - 1]}
          backDown={walked?.ids[walked.at + 1]}
          onStep={step}
          onZoom={(change) => setZoom((z) => clampZoom(change(z)))}
          onCenter={centerOn}
        />
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
// when the viewer has no relation to them.
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
  centered,
  paused,
  flyFrom,
  zoom,
  recentered,
  backUp,
  backDown,
  onStep,
  onZoom,
  onCenter,
}: {
  people: Map<string, TreePerson>
  family: FamilyLinks
  view: FocusView
  focus: string
  viewerId: string | null
  relations: Map<string, string>
  centered: boolean
  paused: boolean
  flyFrom: { to: string; rects: Map<string, DOMRect> } | null
  zoom: number
  recentered: number
  backUp?: string
  backDown?: string
  onStep: (by: 'up' | 'down', to: string) => void
  onZoom: (change: (zoom: number) => number) => void
  onCenter: (id: string) => void
}) {
  const first = (id: string) => people.get(id)!.first_name
  const you = focus === viewerId
  const whose = (words: string) => (you ? `Your ${words}` : `${first(focus)}’s ${words}`)

  // The card under the mouse or keyboard focus: lifted, with the lines to the focus person in green.
  // at: when the lift began, so the green runs to a schedule that stays put while it lasts.
  const [lift, setLift] = useState<{ id: string; at: number } | null>(null)
  const hover = lift?.id ?? null
  const [lines, setLines] = useState<{ width: number; height: number; paths: Line[] }>({ width: 0, height: 0, paths: [] })
  // When each line was first drawn, by key: its draw-in animation starts as it mounts, after a refetch too.
  const [lineStarts, setLineStarts] = useState<ReadonlyMap<string, number>>(new Map())
  const reduced = useReducedMotion()
  const still = reduced || paused
  // Fixed for this view: flying only when centered on from another view, not on opening the page or when the focus
  // falls back by itself (someone left the tree), where the cards' old places would be stale.
  const [from] = useState(() => (!still && flyFrom?.to === focus && flyFrom.rects.size > 0 ? flyFrom.rects : null))
  const fly = from !== null
  const chainLines = chainTo(view, focus, hover)
  const chain = new Map(chainLines.map((l, i) => [l.key, { ...l, at: i }]))
  // Each green line starts once its own line has drawn in and the one before it has run, timed from the lift.
  const greenDelays = new Map<string, number>()
  if (lift) {
    let free = lift.at
    for (const l of chainLines) {
      const base = lines.paths.find((p) => p.key === l.key)
      const mounted = lineStarts.get(l.key)
      const start = Math.max(free, base && mounted !== undefined ? mounted + base.delay + lineDraw : 0)
      greenDelays.set(l.key, start - lift.at)
      free = start + chainStep
    }
  }
  // Everyone the green lines touch stays as they are; the rest fade back.
  const lit = chainLines.length > 0 ? new Set([hover!, focus, ...chainLines.flatMap((l) => l.people)]) : null
  // When the pressed button is gone with the old view, keyboard focus lands on the new focus card,
  // quietly: that focus shouldn't lift it.
  const focusCard = useRef<HTMLButtonElement>(null)
  const quietFocus = useRef(false)

  useEffect(() => {
    if (centered && (!document.activeElement || document.activeElement === document.body)) {
      quietFocus.current = true
      focusCard.current?.focus({ preventScroll: true })
      quietFocus.current = false
    }
  }, [centered])

  const card = (id: string, { sub, former }: { sub?: string; former?: boolean } = {}) => (
    <Person
      key={id}
      person={people.get(id)!}
      sub={sub ?? relations.get(id) ?? ''}
      isFocus={id === focus}
      former={former}
      paused={paused}
      lifted={hover === id}
      dim={lit !== null && !lit.has(id)}
      buttonRef={id === focus ? focusCard : undefined}
      onHover={(on) => {
        if (quietFocus.current) {
          return
        }
        const at = clock()
        setLift((l) => (on ? (l?.id === id ? l : { id, at }) : l?.id === id ? null : l))
      }}
      onCenter={() => onCenter(id)}
    />
  )
  const labelled = (id: string, link: { word: string; alone: string } | null) => {
    const rel = relations.get(id) ?? ''
    // The viewer's relation may already say it ("Foster mother"), but a closer tie wins there ("Grandmother").
    if (!link || rel.toLowerCase().includes(link.word)) {
      return rel
    }
    return rel ? `${rel} · ${link.word}` : link.alone
  }

  const siblings = view.siblings.map(({ id, half, foster }) => {
    const rel = relations.get(id) ?? ''
    // Said from the focus person's side when the viewer's label doesn't say it already: centered on you, a
    // half-sibling is "Half-sister", but a foster sibling may be "Niece".
    const word =
      id === viewerId
        ? null
        : foster
          ? rel.toLowerCase().includes('foster')
            ? null
            : 'foster sibling'
          : half && !you && !(viewerId && family.halfSiblings(viewerId, id))
            ? 'half-sibling'
            : null
    return card(id, { sub: word ? (rel ? `${rel} · ${word}` : word[0]!.toUpperCase() + word.slice(1)) : rel })
  })
  const split = Math.ceil(siblings.length / 2)

  // Top to bottom, the order the generations arrive in; a generation with nobody in it is left out.
  const generations: { key: string; label: string; ids: string[]; body: ReactNode }[] = []
  if (view.grandparents.length > 0) {
    generations.push({
      key: 'grandparents',
      label: 'Grandparents',
      ids: view.grandparents.flatMap((g) => g.ids),
      body: view.grandparents.map((g) => (
        <Group key={g.of} unit={`up-${g.of}`} label={`${first(g.of)}’s parents`}>
          {g.ids.map((id) => card(id, { sub: labelled(id, parentWords[family.linkOf(g.of, id) ?? 'biological']) }))}
        </Group>
      )),
    })
  }
  if (view.parents.length > 0) {
    generations.push({
      key: 'parents',
      label: whose('parents'),
      ids: view.parents,
      body: <Group unit="parents">{view.parents.map((id) => card(id, { sub: labelled(id, parentWords[family.linkOf(focus, id) ?? 'biological']) }))}</Group>,
    })
  }
  generations.push({
    key: 'household',
    label: whose('household & siblings'),
    ids: [focus, ...view.partners, ...view.formerPartners, ...view.siblings.map((x) => x.id)],
    body: (
      <>
        {siblings.slice(0, split)}
        <Group unit="household" household>
          {card(focus)}
          {view.partners.map((id) => card(id))}
          {/* The dashed outline marks a former partner; the words stay the viewer's relation where there is one. */}
          {view.formerPartners.map((id) => card(id, { former: true, sub: relations.get(id) ?? 'Former partner' }))}
        </Group>
        {siblings.slice(split)}
      </>
    ),
  })
  if (view.children.length > 0) {
    generations.push({
      key: 'children',
      label: whose('children'),
      ids: view.children.flatMap((g) => g.ids),
      body: view.children.map((g, i) => (
        <Group
          key={g.with.join(' ') || 'none'}
          unit={`children-${i}`}
          label={g.with.length > 0 && (view.children.length > 1 || view.partners.length + view.formerPartners.length > 1) ? `With ${g.with.map(first).join(' & ')}` : undefined}
        >
          {g.ids.map((id) => card(id, { sub: labelled(id, childWords[family.linkOf(id, focus) ?? 'biological']) }))}
        </Group>
      )),
    })
  }
  if (view.grandchildren.length > 0) {
    generations.push({
      key: 'grandchildren',
      label: 'Grandchildren',
      ids: view.grandchildren.flatMap((g) => g.ids),
      body: view.grandchildren.map((g) => (
        <Group key={g.of} unit={`down-${g.of}`} label={`${first(g.of)}’s children`}>
          {g.ids.map((id) => card(id, { sub: labelled(id, childWords[family.linkOf(id, g.of) ?? 'biological']) }))}
        </Group>
      )),
    })
  }

  // Arrows at the panel's top and bottom naming the family that way, until the tree is moved by hand (owner's calls,
  // 2026-11-21). Each goes one generation: back along the line the arrows came by, else up to a parent (the
  // grandparents are always just above the panel's top: one whose parents' line continues, if any), or down to the
  // child whose children have children.
  const upTo =
    backUp ??
    (view.grandparents.find((g) => g.ids.some((id) => treeParentsOf(family, id).length > 0)) ?? view.grandparents[0])?.of
  const downTo = backDown ?? view.grandchildren.find((g) => g.ids.some((id) => treeChildrenOf(family, id).length > 0))?.of
  const edgeWords = (to: string, by: 'up' | 'down') => {
    const more =
      by === 'up'
        ? treeParentsOf(family, to).length > 0
        : treeChildrenOf(family, to).some((child) => treeChildrenOf(family, child).length > 0)
    return more ? `${first(to)}’s ${by === 'up' ? 'parents' : 'grandchildren'}` : `Back to ${first(to)}`
  }
  const edge = (to: string | undefined, by: 'up' | 'down', where: string, hidden = false) =>
    to &&
    !moved &&
    !hidden && (
      <button type="button" className={`pill ${styles.more} ${styles.edge} ${styles[where]}`} onClick={() => onStep(by, to)}>
        <span aria-hidden="true">{by === 'up' ? '▲' : '▼'} </span>
        <span className={styles.edgeWords}>{edgeWords(to, by)}</span>
        <span className="visually-hidden">: center the tree on {fullName(people.get(to)!)}</span>
      </button>
    )

  const rows = useRef<HTMLDivElement>(null)
  const stage = useRef<HTMLDivElement>(null)
  const panel = useRef<HTMLElement>(null)
  // The panel fills what the window leaves (Main.module.css); the tree is laid out at its width, then scaled and
  // moved inside it.
  const [panelSize, setPanelSize] = useState({ width: 0, height: 0 })
  // Where the cards are on the stage, so dragging can keep some of them, not just the empty stage, in sight.
  const [cards, setCards] = useState<{ left: number; right: number; top: number; bottom: number } | null>(null)
  // Down the stage, where the grandparents' cards end (0 with no grandparents shown): the panel's top edge starts
  // there, so the lines run down from it to the parents and the household. And where the focus person's card is.
  const [anchor, setAnchor] = useState<{ treeTop: number; card: { top: number; bottom: number } | null }>({ treeTop: 0, card: null })
  useLayoutEffect(() => {
    const p = panel.current
    const st = stage.current
    const r = rows.current
    if (!p || !st || !r) {
      return
    }
    const draw = () => {
      const now = clock()
      const paths = measure(st, r, view, focus, fly)
      // A line gone and back later mounts afresh, so only the lines shown keep their times.
      setLineStarts((starts) =>
        paths.length === starts.size && paths.every((l) => starts.has(l.key)) ? starts : new Map(paths.map((l) => [l.key, starts.get(l.key) ?? now])),
      )
      setPanelSize({ width: p.clientWidth, height: p.clientHeight })
      setLines({ width: st.offsetWidth, height: st.offsetHeight, paths })
      setCards(cardBounds(st, r))
      const grandparents = [...r.querySelectorAll<HTMLElement>('[data-unit^="up-"]')].map((g) => offsetBox(g, st).bottom)
      const card = focusCard.current?.closest<HTMLElement>('[data-card]')
      setAnchor({ treeTop: grandparents.length ? Math.max(...grandparents) : 0, card: card ? offsetBox(card, st) : null })
    }
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
  }, [view, focus, fly])

  // Where the tree's top-left corner sits in the panel, moved by dragging and zooming. Unset, it's centered across,
  // and down too while smaller than the panel; centering on anyone, even the person already centered, starts afresh.
  // free: dragged or flung past the limits, shown where it is until it springs back.
  const [drag, setDrag] = useState<{ x: number; y: number; recentered: number; free?: boolean } | null>(null)
  const moved = drag?.recentered === recentered ? drag : null
  const setMoved = (to: { x: number; y: number } | null, free = false) => setDrag(to && { ...to, recentered, free })
  const { width, height } = panelSize
  // Across, the tree is centered. Down, it starts at the bottom of the grandparents' cards, so nothing shows cut
  // off at the panel's top: the lines come down from its edge to the parents, then the household (owner's call).
  // Zoomed in on a short window, that can leave the focus person's card below the panel: then it moves up to show it.
  const resting = (z: number) => {
    const edge = -anchor.treeTop * z
    const { card } = anchor
    const showCard = card ? Math.max(height - cardRoom - card.bottom * z, cardRoom - card.top * z) : edge
    return { x: (width * (1 - z)) / 2, y: Math.min(edge, showCard) }
  }

  // At least 60px of the cards always stays in the panel, at every zoom and after the panel resizes.
  const keepInSight = (x: number, y: number, z: number) => {
    const c = cards ?? { left: 0, right: width, top: 0, bottom: height }
    return {
      x: Math.min(width - 60 - c.left * z, Math.max(60 - c.right * z, x)),
      y: Math.min(height - 60 - c.top * z, Math.max(60 - c.bottom * z, y)),
    }
  }
  const at = moved ? (moved.free ? moved : keepInSight(moved.x, moved.y, zoom)) : resting(zoom)
  // Pointer moves can come several to a render (a pinch's two fingers): each starts from the last one's result.
  const live = useRef({ at, zoom })
  useLayoutEffect(() => {
    live.current = { at, zoom }
  })

  // Once the new view is measured and placed: each card still in view flies from where it was, newcomers fade in, and
  // the groups' boxes fill in around them. The browser's own animations, as the stage's scale is outside Motion's reach.
  const flown = useRef(false)
  useLayoutEffect(() => {
    const r = rows.current
    if (!from || flown.current || !anchor.card || !r) {
      return
    }
    flown.current = true
    const z = live.current.zoom
    let newcomer = 0
    for (const person of r.querySelectorAll<HTMLElement>('[data-person]')) {
      const card = person.querySelector<HTMLElement>('[data-card]')
      if (!card?.animate) {
        continue
      }
      const was = from.get(person.dataset.person!)
      const now = person.getBoundingClientRect()
      if (was) {
        const from = `translate(${(was.left - now.left) / z}px, ${(was.top - now.top) / z}px)`
        card.animate([{ transform: from }, { transform: 'none' }], { id: 'fly', duration: flyMs, easing: flyEasing })
      } else {
        // Capped, so every newcomer has arrived by flyMs, when the lines start.
        card.animate([{ opacity: 0, transform: 'scale(0.9)' }, { opacity: 1, transform: 'none' }], {
          id: 'fly',
          duration: 280,
          delay: 120 + Math.min(newcomer++, 4) * 30,
          easing: 'ease-out',
          fill: 'backwards',
        })
      }
    }
    for (const group of r.querySelectorAll<HTMLElement>('[data-unit]')) {
      const { backgroundColor, borderColor } = getComputedStyle(group)
      group.animate?.([{ backgroundColor: 'transparent', borderColor: 'transparent' }, { backgroundColor, borderColor }], {
        id: 'fly',
        duration: 300,
        delay: 150,
        fill: 'backwards',
      })
    }
    for (const label of r.querySelectorAll<HTMLElement>('[data-label]')) {
      label.animate?.([{ opacity: 0 }, { opacity: 1 }], { id: 'fly', duration: 300, delay: 150, fill: 'backwards' })
    }
  }, [anchor, from])
  // Reduced motion or Pause turned on mid-flight: everything lands at once.
  useEffect(() => {
    if (!still) {
      return
    }
    for (const el of rows.current?.querySelectorAll<HTMLElement>('[data-card], [data-unit], [data-label]') ?? []) {
      el.getAnimations?.().forEach((a) => a.id === 'fly' && a.finish())
    }
  }, [still])

  // Where the page scrolls, keyboard focus landing on the new focus card (preventScroll, above) brings the card into
  // the window too, once it's measured and placed in the panel. Once per view, so a later scroll isn't undone.
  const revealed = useRef(false)
  useEffect(() => {
    const card = focusCard.current
    const p = panel.current
    if (revealed.current || !card || !p || !anchor.card || document.activeElement !== card || !pageScrolls()) {
      return
    }
    revealed.current = true
    // Where the whole card will rest, not the button as it slides in.
    const { y, z } = { y: live.current.at.y, z: live.current.zoom }
    const panelTop = p.getBoundingClientRect().top + p.clientTop
    const top = panelTop + y + anchor.card.top * z
    const bottom = panelTop + y + anchor.card.bottom * z
    if (top < 0 || bottom > window.innerHeight) {
      window.scrollBy({ top: (top + bottom) / 2 - window.innerHeight / 2, behavior: 'instant' })
    }
  }, [anchor])
  const [gliding, setGliding] = useState(false)
  // As the state, but current within one event, which can move the tree several times.
  const glidingNow = useRef(false)
  const glideTimer = useRef<number | undefined>(undefined)
  useEffect(() => () => window.clearTimeout(glideTimer.current), [])
  const endGlide = () => {
    window.clearTimeout(glideTimer.current)
    glidingNow.current = false
    setGliding(false)
  }
  // Not with reduced motion, nor when the tree is already there: nothing would move, and a glide left on would make
  // the next zoom take the resting tree for a moved one.
  const glideBack = (toZoom: number) => {
    const to = resting(toZoom)
    const { at: from, zoom: fromZoom } = live.current
    const there = Math.abs(to.x - from.x) < 0.5 && Math.abs(to.y - from.y) < 0.5 && toZoom === fromZoom
    // Mid-glide, what's stored is already where it's going: a glide on its way there carries on.
    if (there && glidingNow.current) {
      return
    }
    if (reduced || there) {
      endGlide()
      return
    }
    window.clearTimeout(glideTimer.current)
    glidingNow.current = true
    setGliding(true)
    glideTimer.current = window.setTimeout(endGlide, glideMs)
  }
  // A move by hand takes over at once from a glide under way, from where the tree is shown, part way: the position and
  // zoom it's gliding to are already set.
  const takeOverGlide = () => {
    stopFling()
    const st = stage.current
    if (!glidingNow.current || !st || typeof DOMMatrixReadOnly === 'undefined') {
      return
    }
    const shown = getComputedStyle(st).transform
    const m = new DOMMatrixReadOnly(shown === 'none' ? undefined : shown)
    endGlide()
    live.current = { at: { x: m.e, y: m.f }, zoom: m.a }
    setMoved({ x: m.e, y: m.f })
    onZoom(() => m.a)
  }
  // Dragged with one pointer, the tree stretches past its limits and, let go, carries on at the speed it was moving,
  // slowing to a stop and springing back inside them. Not with reduced motion or Pause.
  const fling = useRef<{ raw: { x: number; y: number }; moves: { x: number; y: number; t: number }[] } | null>(null)
  const flingFrame = useRef(0)
  // hold: caught by a press, it stays where it's shown, stretched or not, until let go.
  const stopFling = (hold = false) => {
    if (!flingFrame.current) {
      return
    }
    cancelAnimationFrame(flingFrame.current)
    flingFrame.current = 0
    const { at, zoom } = live.current
    if (!hold) {
      place(at.x, at.y, zoom)
    }
  }
  // Unmounted, or brought back to the middle by centering on the person already centered: a fling stops there.
  useEffect(() => {
    cancelAnimationFrame(flingFrame.current)
    flingFrame.current = 0
  }, [recentered])
  useEffect(() => () => cancelAnimationFrame(flingFrame.current), [])
  const show = (x: number, y: number) => {
    endGlide()
    live.current = { at: { x, y }, zoom: live.current.zoom }
    setMoved({ x, y }, true)
  }
  const dragBy = (dx: number, dy: number, time: number) => {
    if (still) {
      moveBy(dx, dy)
      return
    }
    takeOverGlide()
    // Caught while stretched, the drag carries on from the stretch it shows.
    const startRaw = () => {
      const { at, zoom } = live.current
      const inside = keepInSight(at.x, at.y, zoom)
      return { x: inside.x + (at.x - inside.x) / stretch, y: inside.y + (at.y - inside.y) / stretch }
    }
    const f = (fling.current ??= { raw: startRaw(), moves: [] })
    f.raw = { x: f.raw.x + dx, y: f.raw.y + dy }
    f.moves = [...f.moves.filter((m) => time - m.t < 100), { ...f.raw, t: time }]
    const inside = keepInSight(f.raw.x, f.raw.y, live.current.zoom)
    show(inside.x + (f.raw.x - inside.x) * stretch, inside.y + (f.raw.y - inside.y) * stretch)
  }
  const letGo = (time: number) => {
    cancelAnimationFrame(flingFrame.current)
    flingFrame.current = 0
    const f = fling.current
    fling.current = null
    const { at, zoom } = live.current
    const inside = keepInSight(at.x, at.y, zoom)
    // A press that caught a stretched tree without dragging it still lets it spring back.
    if (!f && (still || Math.hypot(at.x - inside.x, at.y - inside.y) < 0.5)) {
      return
    }
    const moves = f?.moves ?? []
    const recent = moves.filter((m) => time - m.t < 100)
    const [a, b] = [recent[0], recent.at(-1)]
    const ms = a && b ? b.t - a.t : 0
    const cap = (v: number) => Math.max(-60, Math.min(60, v))
    let v = ms > 0 ? { x: cap(((b!.x - a!.x) / ms) * 16), y: cap(((b!.y - a!.y) / ms) * 16) } : { x: 0, y: 0 }
    let pos = { ...live.current.at }
    let last: number | null = null
    const frame = (now: number) => {
      const n = last === null ? 1 : Math.min(4, (now - last) / 16)
      last = now
      // Inside its limits it coasts; past them, its speed outward dies away fast and it eases back, never overshooting.
      const axis = (speed: number, p: number, to: number) => {
        if (Math.abs(p - to) <= 0.5) {
          return { speed: speed * flingFriction ** n, p: p + speed * flingFriction ** n * n }
        }
        const out = Math.sign(p - to) === Math.sign(speed) ? speed * 0.5 ** n : 0
        const next = p + out * n
        return { speed: out, p: to + (next - to) * (1 - flingPull) ** n }
      }
      const inside = keepInSight(pos.x, pos.y, live.current.zoom)
      const ax = axis(v.x, pos.x, inside.x)
      const ay = axis(v.y, pos.y, inside.y)
      v = { x: ax.speed, y: ay.speed }
      pos = { x: ax.p, y: ay.p }
      const settled = Math.hypot(v.x, v.y) < 0.05 && Math.hypot(pos.x - inside.x, pos.y - inside.y) < 0.5
      if (settled) {
        flingFrame.current = 0
        place(pos.x, pos.y, live.current.zoom)
        return
      }
      show(pos.x, pos.y)
      flingFrame.current = requestAnimationFrame(frame)
    }
    flingFrame.current = requestAnimationFrame(frame)
  }
  const place = (x: number, y: number, z: number) => {
    endGlide()
    const to = keepInSight(x, y, z)
    live.current = { at: to, zoom: z }
    setMoved(to)
  }
  const moveBy = (dx: number, dy: number) => {
    takeOverGlide()
    place(live.current.at.x + dx, live.current.at.y + dy, live.current.zoom)
    // A wheel or key mid-drag: the drag carries on from where that put the tree, at no speed yet.
    if (fling.current) {
      fling.current = { raw: { ...live.current.at }, moves: [] }
    }
  }
  // A zoom keeps the point it's made at in place: the pointer or the pinch; the buttons keep a centered tree centered.
  const zoomAround = (change: (z: number) => number, point?: { x: number; y: number }) => {
    takeOverGlide()
    const { at, zoom } = live.current
    const next = clampZoom(change(zoom))
    if (point && next !== zoom) {
      place(point.x - ((point.x - at.x) * next) / zoom, point.y - ((point.y - at.y) * next) / zoom, next)
    } else {
      live.current = { at, zoom: next }
    }
    // A zoom mid-drag: the drag carries on from there, at no speed yet.
    if (fling.current) {
      fling.current = { raw: { ...live.current.at }, moves: [] }
    }
    onZoom(() => next)
  }
  // The − / + buttons and keys: a centered tree stays centered; a moved one zooms about the panel's middle.
  // Mid-glide counts as moved: the glide is taken over where it's shown, which isn't centered.
  const zoomBy = (step: number) => zoomAround((z) => zoomStep(z, step), moved || glidingNow.current ? { x: width / 2, y: height / 2 } : undefined)

  // The wheel: with Ctrl (also how a trackpad pinch arrives) it zooms, which React's wheel listener can't stop the
  // page doing; without, it moves the tree, as the page itself doesn't scroll.
  const wheelActions = useRef({ zoomAround, moveBy })
  useLayoutEffect(() => {
    wheelActions.current = { zoomAround, moveBy }
  })
  useEffect(() => {
    const p = panel.current
    if (!p) {
      return
    }
    const wheel = (event: WheelEvent) => {
      if (!event.ctrlKey && pageScrolls()) {
        return
      }
      event.preventDefault()
      if (!event.ctrlKey) {
        const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? 16 : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? p.clientHeight : 1
        wheelActions.current.moveBy(-event.deltaX * unit, -event.deltaY * unit)
        return
      }
      const box = p.getBoundingClientRect()
      wheelActions.current.zoomAround((z) => z * Math.exp(-event.deltaY / 200), { x: event.clientX - box.left, y: event.clientY - box.top })
    }
    p.addEventListener('wheel', wheel, { passive: false })
    return () => p.removeEventListener('wheel', wheel)
  }, [])

  // Dragging moves the tree, from a card too: past a few pixels it's a drag, not a click. One finger moves it, as a
  // mouse does (only sideways where the page scrolls); two pinch it and move it (touch-action on .panel).
  const pointers = useRef(new Map<number, { x: number; y: number }>())
  const dragged = useRef(false)
  const start = useRef<{ x: number; y: number } | null>(null)
  const middle = () => {
    const [a, b] = [...pointers.current.values()]
    return a && b ? { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, spread: Math.hypot(a.x - b.x, a.y - b.y) } : null
  }
  const onPointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.pointerType === 'mouse' && event.button !== 0) {
      return
    }
    stopFling(true)
    takeOverGlide()
    pointers.current.set(event.pointerId, { x: event.clientX, y: event.clientY })
    start.current = { x: event.clientX, y: event.clientY }
    dragged.current = false
  }
  const onPointerMove = (event: ReactPointerEvent<HTMLElement>) => {
    const last = pointers.current.get(event.pointerId)
    if (!last) {
      return
    }
    const p = event.currentTarget
    const before = middle()
    pointers.current.set(event.pointerId, { x: event.clientX, y: event.clientY })
    const after = middle()
    if (before && after) {
      fling.current = null
      dragged.current = true
      // The move and the zoom in one step: two separate updates from the same position would drop the move.
      const box = p.getBoundingClientRect()
      const point = { x: after.x - box.left, y: after.y - box.top }
      const { at, zoom } = live.current
      const next = before.spread > 0 ? clampZoom((zoom * after.spread) / before.spread) : zoom
      const x = at.x + after.x - before.x
      const y = at.y + after.y - before.y
      place(point.x - ((point.x - x) * next) / zoom, point.y - ((point.y - y) * next) / zoom, next)
      onZoom(() => next)
      return
    }
    if (!dragged.current && start.current && Math.hypot(event.clientX - start.current.x, event.clientY - start.current.y) > 5) {
      dragged.current = true
      p.setPointerCapture?.(event.pointerId)
      // Looking around the tree, not at the card the drag began on: nobody stays faded.
      setLift(null)
    }
    if (dragged.current) {
      dragBy(event.clientX - last.x, event.pointerType === 'touch' && pageScrolls() ? 0 : event.clientY - last.y, event.timeStamp)
    }
  }
  // Anything else the person does stops a throw where it is. A press in the panel, and the focus it gives a card, are
  // the panel's own: they catch it instead.
  const stopNow = useRef(stopFling)
  // Stopped and back inside the limits, from a throw or a stretch held by a drag or a caught spring-back.
  const settle = () => {
    stopFling()
    if (moved?.free) {
      const { at, zoom } = live.current
      place(at.x, at.y, zoom)
    }
  }
  const settleNow = useRef(settle)
  useLayoutEffect(() => {
    stopNow.current = stopFling
    settleNow.current = settle
  })
  // Reduced motion or Pause turned on mid-throw or mid-stretch: the tree settles inside the limits.
  useEffect(() => {
    if (still) {
      fling.current = null
      settleNow.current()
    }
  }, [still])
  useEffect(() => {
    const stop = (event: Event) => {
      const inPanel = panel.current?.contains(event.target as Node)
      if (inPanel && (event.type === 'pointerdown' || (event.type === 'focusin' && pointers.current.size > 0))) {
        return
      }
      stopNow.current()
    }
    const types = ['wheel', 'keydown', 'focusin', 'pointerdown'] as const
    types.forEach((type) => window.addEventListener(type, stop, { capture: true, passive: true }))
    return () => types.forEach((type) => window.removeEventListener(type, stop, { capture: true }))
  }, [])
  // On the window: a press released outside the panel before it became a drag never reaches the panel.
  // Only this panel's own presses: another pointer's release mustn't start a second spring-back.
  const letGoNow = useRef(letGo)
  useLayoutEffect(() => {
    letGoNow.current = letGo
  })
  useEffect(() => {
    const end = (event: PointerEvent) => {
      if (!pointers.current.delete(event.pointerId)) {
        return
      }
      if (pointers.current.size === 0) {
        letGoNow.current(event.timeStamp)
      }
    }
    window.addEventListener('pointerup', end)
    window.addEventListener('pointercancel', end)
    return () => {
      window.removeEventListener('pointerup', end)
      window.removeEventListener('pointercancel', end)
    }
  }, [])

  return (
    <div className={styles.frame}>
      {/* Not while the tree rests moved up to show the focus card: a parent's card can then run under it. */}
      {edge(upTo, 'up', 'edgeTop', at.y < -anchor.treeTop * zoom - 0.5)}
      <section
        ref={panel}
        aria-label={`Family tree centered on ${you ? 'you' : fullName(people.get(focus)!)}`}
        className={styles.panel}
        style={
          {
            '--dots': `${dotGrid * zoom}px`,
            '--dots-at': `${at.x}px ${at.y}px`,
          } as CSSProperties
        }
        onPointerDown={onPointerDown}
        // From a card or link in the tree: the arrow keys move it, + and − zoom it.
        onKeyDown={(event) => {
          if (event.altKey || event.ctrlKey || event.metaKey) {
            return
          }
          const move = ({ ArrowLeft: [keyStep, 0], ArrowRight: [-keyStep, 0], ArrowUp: [0, keyStep], ArrowDown: [0, -keyStep] } as Record<string, [number, number]>)[event.key]
          const zoomStepBy = event.key === '+' || event.key === '=' ? 0.1 : event.key === '-' || event.key === '_' ? -0.1 : 0
          if (move) {
            moveBy(...move)
          } else if (zoomStepBy) {
            zoomBy(zoomStepBy)
          } else {
            return
          }
          event.preventDefault()
        }}
        onPointerMove={onPointerMove}
        // A drag that began on a card doesn't also center the tree on them. Only a pointer's click: Enter on a
        // card clicks it with no pointer (detail 0), and must still work after a drag the browser canceled.
        onFocus={(event) => {
          // A card tabbed to outside the panel is brought in by moving the tree, with a little room around it.
          // Not the page's own focus on the new person's card, and not before the tree is measured.
          const card = (event.target as HTMLElement).closest('[data-card]')
          if (!card || quietFocus.current || width === 0 || height === 0) {
            return
          }
          const box = card.getBoundingClientRect()
          const view = event.currentTarget.getBoundingClientRect()
          if (view.width === 0) {
            return
          }
          const shift = (start: number, end: number, from: number, to: number) =>
            start < from + 16 ? from + 16 - start : end > to - 16 ? to - 16 - end : 0
          const dx = shift(box.left, box.right, view.left, view.right)
          const dy = shift(box.top, box.bottom, view.top, view.bottom)
          if (dx || dy) {
            moveBy(dx, dy)
          }
        }}
        onScroll={(event) => {
          // The browser scrolls the clipped panel to show a card tabbed to: make that a move of the tree instead,
          // which Recenter can undo.
          const el = event.currentTarget
          if (el.scrollLeft || el.scrollTop) {
            moveBy(-el.scrollLeft, -el.scrollTop)
            el.scrollLeft = 0
            el.scrollTop = 0
          }
        }}
        onClickCapture={(event) => {
          if (dragged.current && event.detail > 0) {
            event.stopPropagation()
            event.preventDefault()
            dragged.current = false
          }
        }}
      >
        {/* The tree and its dots stay inside the frame; the panel's own corner is the stage's, so nothing moves. */}
        <div className={styles.inside}>
          <div aria-hidden="true" className={gliding ? `${styles.paper} ${styles.glide}` : styles.paper} />
          <div
            ref={stage}
            data-stage
            className={gliding ? `${styles.stage} ${styles.glide}` : styles.stage}
            style={{
              width: width || undefined,
              transform: at.x === 0 && at.y === 0 && zoom === 1 ? undefined : `translate(${at.x}px, ${at.y}px) scale(${zoom})`,
            }}
          >
            <div aria-hidden="true" className={still ? `${styles.lines} ${styles.still}` : styles.lines}>
              <svg width={lines.width} height={lines.height}>
                {lines.paths.map((l) => (
                  <path key={l.key} data-line={l.key} d={l.d} pathLength={100} style={{ animationDelay: `${l.delay}ms` }} />
                ))}
                {/* Drawn again on top, so a line it shares a stretch with can't cover the green. Mounted with the rest, so
                    a line still drawing in turns green as it draws. */}
                {lines.paths.map((l) => {
                  const green = chain.get(l.key)
                  return (
                    <path
                      // Mounted afresh for each lifted card, so its green runs again in order.
                      key={`over:${l.key}:${green ? lift!.at : ''}`}
                      data-chain={green ? l.key : undefined}
                      d={l.d}
                      pathLength={100}
                      className={green ? `${styles.over} ${styles.chain} ${green.back ? styles.chainBack : ''}` : styles.over}
                      style={{ animationDelay: `${green ? Math.round(greenDelays.get(l.key) ?? 0) : l.delay}ms` }}
                    />
                  )
                })}
              </svg>
            </div>
            {/* Opening the page, the generations arrive in turn too. */}
            <div
              ref={rows}
              className={fly ? styles.rows : `${styles.rows} ${styles.arrive}`}
              style={rowsStyle}
            >
              {generations.map((g, row) => (
                <Generation
                  key={g.key}
                  label={g.label}
                  row={row}
                  raised={hover !== null && g.ids.includes(hover)}
                  spaced={g.key === 'parents' && generations[row - 1]?.key === 'grandparents'}
                >
                  {g.body}
                </Generation>
              ))}
            </div>
          </div>
        </div>
      </section>
      {edge(downTo, 'down', 'edgeBottom')}
      <div role="group" aria-label="Zoom" className={styles.zoom}>
        <button type="button" aria-label="Zoom out" className={styles.zoomButton} disabled={zoom <= minZoom} onClick={() => zoomBy(-0.1)}>
          −
        </button>
        <button type="button" aria-label={`Reset zoom, now ${Math.round(zoom * 100)}%`} className={styles.zoomLevel} onClick={() => {
            stopFling()
            glideBack(1)
            setMoved(null)
            onZoom(() => 1)
          }}>
          {Math.round(zoom * 100)}%
        </button>
        <button type="button" aria-label="Zoom in" className={styles.zoomButton} disabled={zoom >= maxZoom} onClick={() => zoomBy(0.1)}>
          +
        </button>
        <button
          type="button"
          className={styles.recenter}
          disabled={!moved}
          onClick={() => {
            stopFling()
            glideBack(zoom)
            setMoved(null)
          }}
        >
          Recenter
        </button>
      </div>
    </div>
  )
}

// spaced: the parents' row under the grandparents, where the panel's top edge starts: a longer line down to it keeps
// its label clear of the ▲ arrow.
function Generation({ label, row, raised, spaced, children }: { label: string; row: number; raised: boolean; spaced: boolean; children: ReactNode }) {
  const classes = [styles.generation, raised && styles.raised, spaced && styles.spaced].filter(Boolean).join(' ')
  return (
    <div data-row={row} className={classes} style={{ animationDelay: `${row * rowStagger}ms` }}>
      <span data-label className={styles.generationLabel}>
        {label}
      </span>
      <div className={styles.row}>{children}</div>
    </div>
  )
}

function Group({ unit, label, household, children }: { unit: string; label?: string; household?: boolean; children: ReactNode }) {
  return (
    <div data-unit={unit} className={household ? `${styles.group} ${styles.household}` : styles.group}>
      {label && (
        <span data-label className={styles.groupLabel}>
          {label}
        </span>
      )}
      <div className={styles.groupRow}>{children}</div>
    </div>
  )
}

function Person({
  person,
  sub,
  isFocus,
  former,
  paused,
  lifted,
  dim,
  buttonRef,
  onHover,
  onCenter,
}: {
  person: TreePerson
  sub: string
  isFocus: boolean
  former?: boolean
  paused: boolean
  lifted: boolean
  dim: boolean
  buttonRef?: Ref<HTMLButtonElement>
  onHover: (on: boolean) => void
  onCenter: () => void
}) {
  const classes = [styles.card, isFocus && styles.focusCard, former && styles.former, lifted && styles.lifted, dim && styles.dim].filter(Boolean).join(' ')
  return (
    <div
      data-person={person.id}
      className={styles.person}
      onPointerEnter={(event) => event.pointerType === 'mouse' && onHover(true)}
      onPointerLeave={(event) => event.pointerType === 'mouse' && onHover(false)}
      onFocus={() => onHover(true)}
      onBlur={(event) => !event.currentTarget.contains(event.relatedTarget) && onHover(false)}
    >
      <div data-card className={classes}>
        <button ref={buttonRef} type="button" className={styles.center} onClick={onCenter}>
          <span aria-hidden="true" className={isFocus ? `${styles.avatar} ${styles.focusAvatar}` : styles.avatar}>
            {initials(person)}
            <PhotoFade person={person} paused={paused} />
          </span>
          <span className={styles.name}>{fullName(person)}</span>
          {sub && <span className={styles.sub}>{sub}</span>}
        </button>
        {/* Not draggable as a link, so a drag that starts on it moves the tree. */}
        <Link to={`/people/${person.id}`} draggable={false} className={styles.profileLink}>
          View profile <span className="visually-hidden">of {fullName(person)}</span> ›
        </Link>
      </div>
    </div>
  )
}

type Line = { key: string; d: string; delay: number }

function cardBounds(stage: HTMLElement, rows: HTMLElement) {
  const boxes = [...rows.querySelectorAll<HTMLElement>('[data-card]')].map((el) => ({ el, ...offsetBox(el, stage) }))
  if (boxes.length === 0) {
    return null
  }
  return {
    left: Math.min(...boxes.map((b) => b.x - b.el.offsetWidth / 2)),
    right: Math.max(...boxes.map((b) => b.x + b.el.offsetWidth / 2)),
    top: Math.min(...boxes.map((b) => b.top)),
    bottom: Math.max(...boxes.map((b) => b.bottom)),
  }
}

// Where an element rests on the stage: offsets, not screen boxes, so neither the slide-in nor the zoom skews it.
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
function measure(panel: HTMLElement, rows: HTMLElement, view: FocusView, focus: string, fly: boolean): Line[] {
  const box = (el: HTMLElement) => offsetBox(el, panel)
  const unit = (name: string) => rows.querySelector<HTMLElement>(`[data-unit="${name}"]`)
  const card = (id: string) => rows.querySelector<HTMLElement>(`[data-person="${id}"] [data-card]`)
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
  const delay = (to: HTMLElement) => {
    const row = Number(to.closest<HTMLElement>('[data-row]')?.dataset.row ?? 0)
    return fly ? flyMs + row * 40 : row * rowStagger + rowLanded
  }
  // The joining line runs just above the cards it feeds, so lines from the same parents share it; on a row
  // that wraps, the second line of cards gets its own, in the gap between the two.
  return drawn.map((l) => {
    const f = box(l.from)
    const t = box(l.to)
    const m = t.top - 12
    const dx = t.x - f.x
    if (Math.abs(dx) < 1) {
      return { key: l.key, d: `M${f.x} ${f.bottom} V${t.top}`, delay: delay(l.to) }
    }
    const r = Math.max(0, Math.min(10, Math.abs(dx) / 2, m - f.bottom, t.top - m))
    const s = Math.sign(dx)
    return {
      key: l.key,
      d: `M${f.x} ${f.bottom} V${m - r} Q${f.x} ${m} ${f.x + s * r} ${m} H${t.x - s * r} Q${t.x} ${m} ${t.x} ${m + r} V${t.top}`,
      delay: delay(l.to),
    }
  })
}

// The lines from someone to the focus person, in order, by measure()'s keys; a partner shares the household, so has none.
// back: the person is below, so the line fills from its child end. people: everyone the line joins.
type ChainLine = { key: string; back: boolean; people: string[] }
function chainTo(view: FocusView, focus: string, id: string | null): ChainLine[] {
  if (!id || id === focus) {
    return []
  }
  const household = [focus, ...view.partners, ...view.formerPartners]
  const toFocus = { key: `sib:${focus}`, back: false, people: [...view.parents, ...household] }
  const childGroup = (child: string) => view.children.findIndex((g) => g.ids.includes(child))
  const kids = (i: number) => ({ key: `kids:${i}`, back: true, people: [...household, ...view.children[i]!.ids] })
  const grandparent = view.grandparents.find((g) => g.ids.includes(id))
  if (grandparent) {
    return [{ key: `up:${grandparent.of}`, back: false, people: [...grandparent.ids, grandparent.of] }, toFocus]
  }
  if (view.parents.includes(id)) {
    return [toFocus]
  }
  if (view.siblings.some((s) => s.id === id)) {
    return [{ key: `sib:${id}`, back: true, people: [...view.parents, id] }, toFocus]
  }
  if (childGroup(id) >= 0) {
    return [kids(childGroup(id))]
  }
  const grandchild = view.grandchildren.find((g) => g.ids.includes(id))
  return grandchild ? [{ key: `down:${grandchild.of}`, back: true, people: [grandchild.of, ...grandchild.ids] }, kids(childGroup(grandchild.of))] : []
}

// Where each card is shown, by person: its wrapper's box, unskewed by a lift, unless the card is still flying.
function cardRects() {
  return new Map(
    [...document.querySelectorAll<HTMLElement>('[data-stage] [data-person]')].map((el) => {
      const card = el.querySelector<HTMLElement>('[data-card]')
      const flying = card?.getAnimations?.().some((a) => a.id === 'fly')
      return [el.dataset.person!, (flying ? card! : el).getBoundingClientRect()]
    }),
  )
}
