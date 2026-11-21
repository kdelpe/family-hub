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
const tipText = 'Click or tap a name to center the tree on them · drag to move it'
// Each generation arrives this many ms after the one above and has landed after rowLanded (handed to
// Main.module.css as --row-landed); a line draws once the generation it reaches has landed (design-notes §4.2).
const rowStagger = 80
const rowLanded = 400
const rowsStyle = { '--row-landed': `${rowLanded}ms` } as CSSProperties

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

  // Until someone is picked, the tree centers where it opened: on the person a profile's "Center the tree on …"
  // asked for, else you. Someone since removed from the tree is dropped.
  const [params] = useSearchParams()
  const requested = params.get('person')
  const start = requested && people.has(requested) ? requested : viewerId
  const [picked, setPicked] = useState<string | null>(null)
  const focus = [picked, start].find((id) => id && people.has(id)) ?? null
  const view = useMemo(() => (family && focus ? focusView(family, focus) : null), [family, focus])
  const [direction, setDirection] = useState<Direction | null>(null)
  const [status, setStatus] = useState('')
  // Counts centerings on the person already centered, which bring a moved tree back to the middle too.
  const [recentered, setRecentered] = useState(0)

  const centerOn = (to: string) => {
    setLine(null)
    if (to === focus) {
      setRecentered((n) => n + 1)
      return
    }
    if (focus && view) {
      const above = view.parents.includes(to) || view.grandparents.some((g) => g.ids.includes(to))
      const below = view.children.some((g) => g.ids.includes(to)) || view.grandchildren.some((g) => g.ids.includes(to))
      setDirection(above ? 'up' : below ? 'down' : 'side')
      setStatus(`Family tree centered on ${to === viewerId ? 'you' : fullName(people.get(to)!)}`)
    } else if (!focus) {
      // Sideways, as from nowhere in particular; it also lands keyboard focus on the tree once the search clears.
      setDirection('side')
      setStatus(`Family tree centered on ${fullName(people.get(to)!)}`)
    }
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

  // Paused from the People page or Home, which keep the button (design-notes §6.4).
  const [paused] = useMotionPaused()
  const [zoom, setZoom] = useState(1)

  return (
    <main className={styles.page}>
      <title>Family tree — Pluviose-Monestime Family Hub</title>
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
        <>
          {/* On phones, where the panel's corner is the zoom control's. */}
          <p className={styles.tipAbove}>{tipText}</p>
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
            zoom={zoom}
            recentered={recentered}
            backUp={walked?.ids[walked.at - 1]}
            backDown={walked?.ids[walked.at + 1]}
            onStep={step}
            onZoom={(change) => setZoom((z) => clampZoom(change(z)))}
            onCenter={centerOn}
          />
        </>
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
  direction,
  paused,
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
  direction: Direction | null
  paused: boolean
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
  const [hover, setHover] = useState<string | null>(null)
  // When the pressed button is gone with the old view, keyboard focus lands on the new focus card,
  // quietly: that focus shouldn't lift it.
  const focusCard = useRef<HTMLButtonElement>(null)
  const quietFocus = useRef(false)

  useEffect(() => {
    if (direction && (!document.activeElement || document.activeElement === document.body)) {
      quietFocus.current = true
      focusCard.current?.focus({ preventScroll: true })
      quietFocus.current = false
    }
  }, [direction])


  const card = (id: string, { sub, former }: { sub?: string; former?: boolean } = {}) => (
    <Person
      key={id}
      person={people.get(id)!}
      sub={sub ?? relations.get(id) ?? ''}
      isFocus={id === focus}
      former={former}
      paused={paused}
      lifted={hover === id}
      buttonRef={id === focus ? focusCard : undefined}
      onHover={(on) => !quietFocus.current && setHover((h) => (on ? id : h === id ? null : h))}
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
  const chain = new Set(chainTo(view, focus, hover))

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

  // Plain arrows at the panel's top and bottom saying there's more family that way, until the tree is moved by hand
  // (owner's call, 2026-11-21). Each goes one generation: back along the line the arrows came by, else along the
  // line that continues, to the parent whose parents have family above or the child whose children have children.
  const upTo = backUp ?? view.grandparents.find((g) => g.ids.some((id) => treeParentsOf(family, id).length > 0))?.of
  const downTo = backDown ?? view.grandchildren.find((g) => g.ids.some((id) => treeChildrenOf(family, id).length > 0))?.of
  const edge = (to: string | undefined, by: 'up' | 'down', where: string) =>
    to &&
    !moved && (
      <button type="button" className={`pill ${styles.more} ${styles.edge} ${styles[where]}`} onClick={() => onStep(by, to)}>
        <span aria-hidden="true">{by === 'up' ? '▲' : '▼'} </span>more
        <span className="visually-hidden">: center the tree on {fullName(people.get(to)!)}</span>
      </button>
    )

  const rows = useRef<HTMLDivElement>(null)
  const stage = useRef<HTMLDivElement>(null)
  const panel = useRef<HTMLElement>(null)
  const [lines, setLines] = useState<{ width: number; height: number; paths: Line[] }>({ width: 0, height: 0, paths: [] })
  // The tree is laid out at the panel's width, then scaled inside it: the panel keeps its size at every zoom,
  // and scrolls when zoomed in.
  const [panelWidth, setPanelWidth] = useState(0)
  // Where the cards are on the stage, so dragging can keep some of them, not just the empty stage, in sight.
  const [cards, setCards] = useState<{ left: number; right: number; top: number; bottom: number } | null>(null)
  useLayoutEffect(() => {
    const p = panel.current
    const st = stage.current
    const r = rows.current
    if (!p || !st || !r) {
      return
    }
    const draw = () => {
      setPanelWidth(p.clientWidth)
      setLines({ width: st.offsetWidth, height: st.offsetHeight, paths: measure(st, r, view, focus) })
      setCards(cardBounds(st, r))
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
  }, [view, focus])

  // Where the tree's top-left corner sits in the panel, moved by dragging and zooming. Unset, it's centered across,
  // and down too while smaller than the panel; centering on anyone, even the person already centered, starts afresh.
  const [drag, setDrag] = useState<{ x: number; y: number; recentered: number } | null>(null)
  const moved = drag?.recentered === recentered ? drag : null
  const setMoved = (to: { x: number; y: number } | null) => setDrag(to && { ...to, recentered })
  const width = panelWidth
  const height = lines.height
  const centered = (z: number) => ({ x: (width * (1 - z)) / 2, y: z < 1 ? (height * (1 - z)) / 2 : 0 })
  // At least 60px of the cards always stays in the panel, at every zoom and after the panel resizes.
  const keepInSight = (x: number, y: number, z: number) => {
    const c = cards ?? { left: 0, right: width, top: 0, bottom: height }
    return {
      x: Math.min(width - 60 - c.left * z, Math.max(60 - c.right * z, x)),
      y: Math.min(height - 60 - c.top * z, Math.max(60 - c.bottom * z, y)),
    }
  }
  const at = moved ? keepInSight(moved.x, moved.y, zoom) : centered(zoom)
  // Pointer moves can come several to a render (a pinch's two fingers): each starts from the last one's result.
  const live = useRef({ at, zoom })
  useLayoutEffect(() => {
    live.current = { at, zoom }
  })
  const place = (x: number, y: number, z: number) => {
    const to = keepInSight(x, y, z)
    live.current = { at: to, zoom: z }
    setMoved(to)
  }
  const moveBy = (dx: number, dy: number) => place(live.current.at.x + dx, live.current.at.y + dy, live.current.zoom)
  // A zoom keeps the point it's made at in place: the pointer or the pinch; the buttons keep a centered tree centered.
  const zoomAround = (change: (z: number) => number, point?: { x: number; y: number }) => {
    const { at, zoom } = live.current
    const next = clampZoom(change(zoom))
    if (point && next !== zoom) {
      place(point.x - ((point.x - at.x) * next) / zoom, point.y - ((point.y - at.y) * next) / zoom, next)
    } else {
      live.current = { at, zoom: next }
    }
    onZoom(() => next)
  }

  // The focus person's card is brought into view once the panel has its height: at once on opening the page,
  // smoothly after centering on someone. Once per view, so a later drag or zoom doesn't pull the page back.
  const reducedMotion = useReducedMotion()
  const shown = useRef(false)
  useEffect(() => {
    const card = focusCard.current?.closest('[data-card]')
    if (shown.current || !card || height === 0) {
      return
    }
    shown.current = true
    const box = card.getBoundingClientRect()
    if (box.top < 0 || box.bottom > window.innerHeight) {
      // The window only: scrollIntoView would also scroll the clipped panel, which the drag position doesn't know of.
      const top = window.scrollY + box.top + box.height / 2 - window.innerHeight / 2
      window.scrollTo({ top, behavior: direction && !reducedMotion ? 'smooth' : 'instant' })
    }
  }, [height, direction, reducedMotion])

  // Ctrl + scroll, which is also how a trackpad pinch arrives; React's wheel listener can't prevent the page zoom.
  const wheelZoom = useRef(zoomAround)
  useLayoutEffect(() => {
    wheelZoom.current = zoomAround
  })
  useEffect(() => {
    const p = panel.current
    if (!p) {
      return
    }
    const wheel = (event: WheelEvent) => {
      if (!event.ctrlKey) {
        return
      }
      event.preventDefault()
      const box = p.getBoundingClientRect()
      wheelZoom.current((z) => z * Math.exp(-event.deltaY / 200), { x: event.clientX - box.left, y: event.clientY - box.top })
    }
    p.addEventListener('wheel', wheel, { passive: false })
    return () => p.removeEventListener('wheel', wheel)
  }, [])

  // Dragging moves the tree, from a card too: past a few pixels it's a drag, not a click. Two fingers pinch it and
  // move it; one finger moves it sideways, leaving up and down to the page (touch-action on .panel).
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
    }
    if (dragged.current) {
      moveBy(event.clientX - last.x, event.pointerType === 'touch' ? 0 : event.clientY - last.y)
    }
  }
  // On the window: a press released outside the panel before it became a drag never reaches the panel.
  useEffect(() => {
    const end = (event: PointerEvent) => pointers.current.delete(event.pointerId)
    window.addEventListener('pointerup', end)
    window.addEventListener('pointercancel', end)
    return () => {
      window.removeEventListener('pointerup', end)
      window.removeEventListener('pointercancel', end)
    }
  }, [])

  return (
    <div className={styles.frame}>
      <p className={styles.tip}>{tipText}</p>
      {edge(upTo, 'up', 'edgeTop')}
      <section
        ref={panel}
        aria-label={`Family tree centered on ${you ? 'you' : fullName(people.get(focus)!)}`}
        className={styles.panel}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        // A drag that began on a card doesn't also center the tree on them. Only a pointer's click: Enter on a
        // card clicks it with no pointer (detail 0), and must still work after a drag the browser canceled.
        onClickCapture={(event) => {
          if (dragged.current && event.detail > 0) {
            event.stopPropagation()
            event.preventDefault()
            dragged.current = false
          }
        }}
        style={{ height: height || undefined }}
      >
        <div
          ref={stage}
          data-stage
          className={styles.stage}
          style={{
            width: width || undefined,
            transform: at.x === 0 && at.y === 0 && zoom === 1 ? undefined : `translate(${at.x}px, ${at.y}px) scale(${zoom})`,
          }}
        >
          <div aria-hidden="true" className={styles.lines}>
            <svg width={lines.width} height={lines.height}>
              {lines.paths.map((l) => (
                <path key={l.key} data-line={l.key} d={l.d} pathLength={100} style={{ animationDelay: `${l.delay}ms` }} />
              ))}
              {/* Drawn again on top, so a line it shares a stretch with can't cover the green. Mounted with the rest, so
                  a line still drawing in turns green as it draws. */}
              {lines.paths.map((l) => (
                <path
                  key={`over:${l.key}`}
                  data-chain={chain.has(l.key) ? l.key : undefined}
                  d={l.d}
                  pathLength={100}
                  className={chain.has(l.key) ? `${styles.over} ${styles.chain}` : styles.over}
                  style={{ animationDelay: `${l.delay}ms` }}
                />
              ))}
            </svg>
          </div>
          {/* Opening the page, the generations arrive in turn too. */}
          <div
            ref={rows}
            className={[styles.rows, styles[direction ?? 'side'], upTo && styles.roomAbove, downTo && styles.roomBelow].filter(Boolean).join(' ')}
            style={rowsStyle}
          >
            {generations.map((g, row) => (
              <Generation key={g.key} label={g.label} row={row} raised={hover !== null && g.ids.includes(hover)}>
                {g.body}
              </Generation>
            ))}
          </div>
        </div>
      </section>
      {edge(downTo, 'down', 'edgeBottom')}
      <div role="group" aria-label="Zoom" className={styles.zoom}>
        <button type="button" aria-label="Zoom out" className={styles.zoomButton} disabled={zoom <= minZoom} onClick={() => zoomAround((z) => zoomStep(z, -0.1), moved ? { x: width / 2, y: height / 2 } : undefined)}>
          −
        </button>
        <button type="button" aria-label={`Reset zoom, now ${Math.round(zoom * 100)}%`} className={styles.zoomLevel} onClick={() => {
            setMoved(null)
            onZoom(() => 1)
          }}>
          {Math.round(zoom * 100)}%
        </button>
        <button type="button" aria-label="Zoom in" className={styles.zoomButton} disabled={zoom >= maxZoom} onClick={() => zoomAround((z) => zoomStep(z, 0.1), moved ? { x: width / 2, y: height / 2 } : undefined)}>
          +
        </button>
        <button type="button" className={styles.recenter} disabled={!moved} onClick={() => setMoved(null)}>
          Recenter
        </button>
      </div>
    </div>
  )
}

function Generation({ label, row, raised, children }: { label: string; row: number; raised: boolean; children: ReactNode }) {
  return (
    <div data-row={row} className={raised ? `${styles.generation} ${styles.raised}` : styles.generation} style={{ animationDelay: `${row * rowStagger}ms` }}>
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
  isFocus,
  former,
  paused,
  lifted,
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
  buttonRef?: Ref<HTMLButtonElement>
  onHover: (on: boolean) => void
  onCenter: () => void
}) {
  const classes = [styles.card, isFocus && styles.focusCard, former && styles.former, lifted && styles.lifted].filter(Boolean).join(' ')
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
function measure(panel: HTMLElement, rows: HTMLElement, view: FocusView, focus: string): Line[] {
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
  const delay = (to: HTMLElement) => Number(to.closest<HTMLElement>('[data-row]')?.dataset.row ?? 0) * rowStagger + rowLanded
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

// The lines joining someone to the focus person, by the keys measure() gives them: a grandparent through
// their child, a sibling through the parents, a grandchild through their parent. A partner shares the household.
function chainTo(view: FocusView, focus: string, id: string | null): string[] {
  if (!id || id === focus) {
    return []
  }
  const childGroup = (child: string) => view.children.findIndex((g) => g.ids.includes(child))
  const grandparent = view.grandparents.find((g) => g.ids.includes(id))
  if (grandparent) {
    return [`up:${grandparent.of}`, `sib:${focus}`]
  }
  if (view.parents.includes(id)) {
    return [`sib:${focus}`]
  }
  if (view.siblings.some((s) => s.id === id)) {
    return [`sib:${id}`, `sib:${focus}`]
  }
  if (childGroup(id) >= 0) {
    return [`kids:${childGroup(id)}`]
  }
  const grandchild = view.grandchildren.find((g) => g.ids.includes(id))
  return grandchild ? [`kids:${childGroup(grandchild.of)}`, `down:${grandchild.of}`] : []
}
