import { type PointerEvent as ReactPointerEvent, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react'

import { type Book as BookData, nameOf, pageNumber, stopOf, stopsOf } from './bookPages'
import { PageContent } from './PageContent'
import styles from './Members.module.css'

type Dir = 'next' | 'prev'
// land: the page an animated turn is bringing into view (a drag's isn't known until it's let go).
type Turn = { dir: Dir; to: number; mode: 'anim' | 'drag' | 'settle'; stops: number[][]; land?: number }
type Drag = {
  id: number
  el: HTMLDivElement
  side: Dir
  corner: boolean
  x0: number
  left: number
  right: number
  on: boolean
  dir: Dir
  to: number
  p: number
  hist: [number, number][]
  stops: number[][]
}
type TabDrag = { nav: HTMLElement; id: number; start: number; cur: number; y0: number; moved: boolean; book: BookData }

// A turn takes 0.9 s (Members.module.css), plus a frame.
const turnMs = 920
// How long the closed cover shows on arrival before it opens itself.
const arrivalMs = 600
// A flick is measured over this last stretch of the drag, so a page held still before letting go isn't flicked.
const flickMs = 100
// The page edges grow as pages pile up, up to this: a big family has thousands of pages.
const maxStack = 18

type Props = {
  book: BookData
  // One page at a time, on a phone.
  single: boolean
  // The page the book is open at (index into book.pages); the stop showing it is what's drawn.
  at: number
  onAt: (index: number) => void
  viewerId: string | null
  paused: boolean
  reduced: boolean
}

// The Members book (design-notes §4.3): one registered property, --fa, turns a page through calc(), sin() and cos()
// in the CSS; a drag writes it straight to the element, with no render per pointer move.
export function Book({ book, single, at, onAt, viewerId, paused, reduced }: Props) {
  const stops = useMemo(() => stopsOf(book, single), [book, single])
  const last = stops.length - 1
  const stop = stopOf(stops, at)
  // A turn or drag belongs to the stops it was made for: a refetch, or a phone turned on its side, cancels one under way.
  const [made, setTurn] = useState<Turn | null>(null)
  const turn = made?.stops === stops ? made : null
  // Like a turn, a scrub belongs to the tabs it started on: a refetch can take letters away.
  const [scrubbed, setScrub] = useState<{ tab: number; top: number; book: BookData } | null>(null)
  const scrub = scrubbed?.book === book ? scrubbed : null
  // What's typed in the page box until it's taken; otherwise the box shows the page the book is open at.
  const [draft, setDraft] = useState<string | null>(null)
  const pageField = useId()
  const pagesRef = useRef<HTMLDivElement>(null)
  const timer = useRef<number | undefined>(undefined)
  const drag = useRef<Drag | null>(null)
  const justDragged = useRef(-Infinity)
  const tabDrag = useRef<TabDrag | null>(null)
  const tabHandled = useRef(-Infinity)
  useEffect(() => () => window.clearTimeout(timer.current), [])
  useEffect(() => {
    window.clearTimeout(timer.current)
    const el = pagesRef.current
    if (el) {
      el.style.transition = 'none'
      el.style.removeProperty('--fa')
    }
  }, [stops])

  const finish = (page: number | null, ms: number) => {
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => {
      const el = pagesRef.current
      if (el) {
        el.style.transition = 'none'
        el.style.removeProperty('--fa')
      }
      if (page !== null) {
        onAt(page)
      }
      setTurn(null)
    }, ms)
  }
  // Lands on the page asked for, not just its spread, so a refetch that moves the spreads keeps that person in view.
  const flipTo = (to: number, page?: number) => {
    if (turn || to < 0 || to > last) {
      return
    }
    if (to === stop) {
      if (page !== undefined) {
        onAt(page)
      }
      return
    }
    // A control on a page turned away (or a corner or button, closing the book) goes with it: the book takes focus, so
    // the arrow keys still turn.
    const active = document.activeElement
    if (active?.closest('[data-page]') || (active?.closest('[data-corner], [data-controls]') && (to === 0 || to === last))) {
      pagesRef.current?.focus({ preventScroll: true })
    }
    const land = page ?? stops[to]![0]!
    if (reduced) {
      onAt(land)
      return
    }
    setTurn({ dir: to > stop ? 'next' : 'prev', to, mode: 'anim', stops, land })
    finish(land, turnMs)
  }

  // Arriving on the page, the book opens itself to the first pages, so it's clear what it is and how it turns
  // (owner's call). Not with reduced motion, and not once the book's been touched.
  const openOnArrival = useRef(() => {})
  useLayoutEffect(() => {
    openOnArrival.current = () => stop === 0 && !turn && flipTo(1)
  })
  useEffect(() => {
    if (reduced) {
      return
    }
    const wait = window.setTimeout(() => openOnArrival.current(), arrivalMs)
    return () => window.clearTimeout(wait)
    // Once, on arrival.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  const goTo = (index: number) => flipTo(stopOf(stops, index), index)

  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    const target = event.target as Element
    if (turn || event.button > 0 || target.closest('a, input, select, textarea, button:not([data-corner])')) {
      return
    }
    // A press on a scrollbar (a long list of contacts) scrolls; it doesn't turn the page.
    if (target instanceof HTMLElement && target.scrollHeight > target.clientHeight && event.nativeEvent.offsetX > target.clientWidth) {
      return
    }
    const el = event.currentTarget
    const box = el.getBoundingClientRect()
    const half: Dir = event.clientX >= box.left + box.width / 2 ? 'next' : 'prev'
    // On a phone a cover turns one way only, wherever it's tapped.
    const side: Dir = single && stop === 0 ? 'next' : single && stop === last ? 'prev' : half
    const to = stop + (side === 'next' ? 1 : -1)
    if (!single && (to < 0 || to > last)) {
      return
    }
    drag.current = {
      id: event.pointerId,
      el,
      side,
      corner: !!target.closest('[data-corner]'),
      x0: event.clientX,
      left: box.left,
      right: box.right,
      on: false,
      dir: side,
      to,
      p: 0,
      hist: [[event.timeStamp, event.clientX]],
      stops,
    }
  }

  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const d = drag.current
    if (!d || event.pointerId !== d.id) {
      return
    }
    if (d.stops !== stops) {
      drag.current = null
      return
    }
    const dx = event.clientX - d.x0
    if (!d.on) {
      // A 6px dead zone keeps a click a click.
      if (Math.abs(dx) < 6) {
        return
      }
      // A spread turns the page grabbed, toward the spine; a single page turns whichever way it's pulled.
      d.dir = single ? (dx < 0 ? 'next' : 'prev') : d.side
      d.to = stop + (d.dir === 'next' ? 1 : -1)
      if ((d.dir === 'next') !== dx < 0 || d.to < 0 || d.to > last) {
        drag.current = null
        return
      }
      d.on = true
      d.el.setPointerCapture?.(d.id)
      // With reduced motion the page doesn't follow the pointer: letting go turns it at once, or not.
      if (!reduced) {
        d.el.style.transition = 'none'
        d.el.style.setProperty('--fa', single && d.dir === 'prev' ? '-180' : '0')
        setTurn({ dir: d.dir, to: d.to, mode: 'drag', stops })
      }
    }
    // Done when the pointer reaches the far edge: the left one turning forward, the right one turning back.
    const far = d.dir === 'next' ? d.left : d.right
    const p = Math.min(1, Math.max(0, dx / (far - d.x0)))
    d.p = p
    if (!reduced) {
      const angle = (Math.acos(1 - 2 * p) * 180) / Math.PI
      const fa = single && d.dir === 'prev' ? angle - 180 : d.dir === 'next' ? -angle : angle
      d.el.style.setProperty('--fa', fa.toFixed(2))
    }
    d.hist.push([event.timeStamp, event.clientX])
    if (d.hist.length > 6) {
      d.hist.shift()
    }
  }

  // A canceled pointer is a touch the browser took over for a scroll: it turns nothing, and a page under way falls back.
  const release = (event: ReactPointerEvent<HTMLDivElement>, canceled: boolean) => {
    const d = drag.current
    if (!d || event.pointerId !== d.id) {
      return
    }
    drag.current = null
    if (d.stops !== stops) {
      return
    }
    if (!d.on) {
      // The corners turn by their own click.
      if (!d.corner && !canceled) {
        flipTo(stop + (d.side === 'next' ? 1 : -1))
      }
      return
    }
    justDragged.current = event.timeStamp
    const recent = [...d.hist, [event.timeStamp, event.clientX] as [number, number]].filter(([t]) => t >= event.timeStamp - flickMs)
    const first = recent[0]!
    const lastMove = recent.at(-1)!
    const speed = lastMove[0] > first[0] ? (lastMove[1] - first[1]) / (lastMove[0] - first[0]) : 0
    const toward = d.dir === 'next' ? -speed : speed
    // Past halfway, or flicked toward the spine: it finishes; otherwise it falls back.
    const done = !canceled && (d.p > 0.5 ? toward > -0.3 : toward > 0.35 && d.p > 0.03)
    const land = done ? stops[d.to]![0]! : null
    if (reduced) {
      // Reduced motion may have come on mid-drag, with the leaf already out.
      d.el.style.transition = 'none'
      d.el.style.removeProperty('--fa')
      setTurn(null)
      if (land !== null) {
        onAt(land)
      }
      return
    }
    const end = single && d.dir === 'prev' ? (done ? 0 : -180) : done ? (d.dir === 'next' ? -180 : 180) : 0
    const ms = Math.round(200 + 460 * (done ? 1 - d.p : d.p))
    d.el.style.transition = `--fa ${ms}ms cubic-bezier(.25,.8,.35,1)`
    d.el.style.setProperty('--fa', String(end))
    setTurn((t) => t && { ...t, mode: 'settle', ...(land !== null && { land }) })
    finish(land, ms + 30)
  }

  // The letter tabs: a tap turns there; pressing and sliding scrubs straight to each letter as it's passed.
  const tabAt = (nav: HTMLElement, y: number) => {
    let best: { i: number; top: number } | null = null
    let distance = Infinity
    nav.querySelectorAll<HTMLButtonElement>('button[data-tab]').forEach((b) => {
      const box = b.getBoundingClientRect()
      const d = Math.abs(y - (box.top + box.height / 2))
      if (d < distance) {
        distance = d
        best = { i: Number(b.dataset.tab), top: b.offsetTop }
      }
    })
    return best as { i: number; top: number } | null
  }

  const tabTargets = [...book.letterStarts.map((l) => l.index), book.memoryStart]

  const cur = stops[stop]!
  const tgtStop = turn ? turn.to : stop
  const tgt = stops[tgtStop]!
  const shown = cur.map((i) => book.pages[i]!)
  const tabs = [
    ...book.letterStarts.map(({ letter }) => ({ label: letter, aria: `Jump to ${letter}`, on: shown.some((p) => p.kind === 'person' && p.letter === letter) })),
    { label: '♡', aria: 'Jump to In Memory', on: shown.some((p) => p.kind === 'memorial' || p.kind === 'divider') },
  ]


  const names = shown.flatMap((p) => {
    const who = nameOf(p)
    return who ? [who.first_name] : []
  })
  const numbers = cur.flatMap((i) => pageNumber(book, i) ?? [])
  // Only pages the book can show: a phone leaves out the Notes page.
  const total = Math.max(...stops.flat().map((i) => Number(pageNumber(book, i) ?? 0)))
  // Where the book is going, once that's known (a click, a button, a drag let go to finish): closing, the buttons and page
  // box go as the cover starts to swing, not when it lands.
  const heading = turn?.land !== undefined ? turn.to : stop
  const closed = heading === 0 || heading === last
  // The box shows the page asked for (a typed page, a person jumped to), from the moment the turn starts; turned to
  // by Previous or Next, the spread's first numbered page.
  const boxAt = turn?.land ?? at
  const boxSpread = stops[stopOf(stops, boxAt)]!
  const boxNumber =
    (boxSpread.includes(boxAt) ? pageNumber(book, boxAt) : null) ?? boxSpread.flatMap((i) => pageNumber(book, i) ?? [])[0] ?? ''
  // Anything typed in the page box is taken on Enter or on leaving it; a number that isn't a page puts it back.
  const goToPage = () => {
    const n = Number(draft)
    setDraft(null)
    if (draft && Number.isInteger(n) && n >= 1 && n <= total) {
      goTo(book.welcome + n - 1)
    }
  }
  let position = numbers.length
    ? `Page${numbers.length > 1 ? `s ${numbers[0]}–${numbers[1]}` : ` ${numbers[0]}`}${names.length ? ` · ${names.join(' & ')}` : ''}`
    : ''
  if (heading === 0) {
    position = 'Click the cover, or drag it, to open the book'
  } else if (heading === last) {
    position = 'The end · click the back cover to go back'
  } else if (shown.some((p) => p.kind === 'endpaperFront')) {
    position = 'Welcome'
  }

  const page = (index: number, place: 'left' | 'right' | 'whole', extra = '') => (
    <div
      // By who's on it, not where: a page's request buttons mustn't pass to whoever lands there after a refetch.
      key={`${place}-${book.keys[index]}`}
      data-page
      className={`${styles.page} ${styles[place]} ${extra} ${book.pages[index]!.kind === 'empty' ? styles.hidden : ''}`}
    >
      <PageContent book={book} index={index} outer={place === 'right' ? 'right' : 'left'} viewerId={viewerId} paused={paused} goTo={goTo} />
    </div>
  )
  const face = (index: number | null, className: string, outer: 'left' | 'right', inner: string, back = false) => (
    <div className={`${styles.face} ${back ? styles.back : ''} ${className}`}>
      <div className={`${styles.inner} ${inner}`}>
        {index !== null && (
          <PageContent book={book} index={index} outer={outer} viewerId={viewerId} paused={paused} goTo={goTo} />
        )}
      </div>
      <span className={styles.shade} />
    </div>
  )

  // The static pages under a turning leaf, and what the leaf carries on each side.
  let statics: number[]
  let front: number | null = null
  let backFace: number | null = null
  if (single) {
    statics = turn?.dir === 'next' ? tgt : cur
    front = turn ? (turn.dir === 'next' ? cur[0]! : tgt[0]!) : null
  } else if (turn?.dir === 'next') {
    statics = [cur[0]!, tgt[1]!]
    front = cur[1]!
    backFace = tgt[0]!
  } else if (turn?.dir === 'prev') {
    statics = [tgt[0]!, cur[1]!]
    front = cur[0]!
    backFace = tgt[1]!
  } else {
    statics = cur
  }
  const hint = stop === 0 && !turn ? styles.coverHint! : ''
  const emptyLeft = !single && book.pages[statics[0]!]!.kind === 'empty'
  const emptyRight = !single && book.pages[statics[1]!]!.kind === 'empty'
  const rigid = [front, backFace].some((i) => i !== null && (book.pages[i]!.kind === 'cover' || book.pages[i]!.kind === 'back'))
  // A single page always turns about its left edge; turning back, it comes in from the left.
  const leafDir: Dir = single ? 'next' : (turn?.dir ?? 'next')

  const turnClass = turn
    ? turn.mode === 'anim'
      ? single && turn.dir === 'prev'
        ? styles.animUnturn
        : turn.dir === 'next'
          ? styles.animNext
          : styles.animPrev
      : styles.dragging
    : ''
  const stackLeft = stop === 0 ? 0 : Math.min(maxStack, 3 + Math.round(stop * 0.8))
  const stackRight = stop === last ? 0 : Math.min(maxStack, 3 + Math.round((last - stop) * 0.8))

  return (
    <div className={`${styles.bookArea} ${paused || reduced ? styles.still : ''} ${single ? styles.single : ''}`}>
      <div className={`${styles.book} ${!single && tgtStop === 0 ? styles.closedStart : ''} ${!single && tgtStop === last ? styles.closedEnd : ''}`}>
        {single ? (
          <span aria-hidden="true" className={`${styles.board} ${styles.boardWhole} ${tgtStop === 0 || tgtStop === last ? styles.gone : ''}`} />
        ) : (
          <>
            <span aria-hidden="true" className={`${styles.board} ${styles.boardLeft} ${tgtStop === 0 ? styles.gone : ''}`} />
            <span aria-hidden="true" className={`${styles.board} ${styles.boardRight} ${tgtStop === last ? styles.gone : ''}`} />
          </>
        )}
        <div
          ref={pagesRef}
          className={`${styles.pages} ${turnClass}`}
          tabIndex={0}
          role="group"
          aria-roledescription="book"
          aria-label="Members book. Click or drag a page, or use the left and right arrow keys, to turn."
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={(event) => release(event, false)}
          onPointerCancel={(event) => release(event, true)}
          onKeyDown={(event) => {
            if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
              event.preventDefault()
              flipTo(stop + (event.key === 'ArrowRight' ? 1 : -1))
            }
          }}
        >
          <span aria-hidden="true" className={styles.stack} style={{ left: -stackLeft, width: stackLeft }} />
          <span aria-hidden="true" className={styles.stack} style={{ right: -stackRight, width: stackRight }} />
          {single
            ? page(statics[0]!, 'whole', hint)
            : [page(statics[0]!, 'left'), page(statics[1]!, 'right', hint)]}
          {!single && <div aria-hidden="true" className={`${styles.spine} ${tgtStop === 0 || tgtStop === last ? styles.gone : ''}`} />}
          {turn && front !== null && (
            // Copies of pages shown elsewhere, mid-turn: kept from screen readers and the keyboard.
            <div aria-hidden="true" inert className={styles.turning}>
              {/* No shadow falls where there's no page, beside a cover. */}
              {!(leafDir === 'next' ? emptyRight : emptyLeft) && (
                <span className={`${styles.cast} ${leafDir === 'next' ? styles.underNext : styles.underPrev}`} />
              )}
              {!single && !(leafDir === 'next' ? emptyLeft : emptyRight) && (
                <span className={`${styles.cast} ${leafDir === 'next' ? styles.landNext : styles.landPrev}`} />
              )}
              <div className={`${styles.leaf} ${leafDir === 'next' ? styles.leafNext : styles.leafPrev} ${rigid ? styles.rigid : ''}`}>
                <div className={`${styles.seg} ${styles.sa}`}>
                  {leafDir === 'next'
                    ? face(front, styles.square!, 'right', `${styles.innerRight} ${styles.saFrontNext}`)
                    : face(front, styles.square!, 'left', `${styles.innerLeft} ${styles.saFrontPrev}`)}
                  {leafDir === 'next'
                    ? face(backFace, styles.square!, 'left', `${styles.innerLeft} ${styles.saBackNext}`, true)
                    : face(backFace, styles.square!, 'right', `${styles.innerRight} ${styles.saBackPrev}`, true)}
                  <div className={`${styles.seg} ${styles.sb}`}>
                    {leafDir === 'next' ? (
                      <>
                        <div className={`${styles.face} ${styles.roundRight}`}>
                          <div className={`${styles.inner} ${styles.innerRight} ${styles.sbFrontNext}`}>
                            <PageContent book={book} index={front} outer="right" viewerId={viewerId} paused={paused} goTo={goTo} />
                          </div>
                          <span className={styles.shade} />
                          <span className={styles.gloss} />
                        </div>
                        {face(backFace, styles.roundLeft!, 'left', `${styles.innerLeft} ${styles.sbBackNext}`, true)}
                      </>
                    ) : (
                      <>
                        <div className={`${styles.face} ${styles.roundLeft}`}>
                          <div className={`${styles.inner} ${styles.innerLeft} ${styles.sbFrontPrev}`}>
                            <PageContent book={book} index={front} outer="left" viewerId={viewerId} paused={paused} goTo={goTo} />
                          </div>
                          <span className={styles.shade} />
                          <span className={styles.gloss} />
                        </div>
                        {face(backFace, styles.roundRight!, 'right', `${styles.innerRight} ${styles.sbBackPrev}`, true)}
                      </>
                    )}
                  </div>
                </div>
              </div>
            </div>
          )}
          {stop !== 0 && stop !== last && (
            <>
              {!single && (
                <button type="button" className={`${styles.corner} ${styles.cornerPrev}`} data-corner="prev" aria-label="Previous page" onClick={(event) => event.timeStamp - justDragged.current > 350 && flipTo(stop - 1)}>
                  <span aria-hidden="true" className={`${styles.ear} ${styles.earLeft}`} />
                </button>
              )}
              <button
                type="button"
                className={`${styles.corner} ${styles.cornerNext}`}
                data-corner="next"
                aria-label="Next page"
                onClick={(event) => event.timeStamp - justDragged.current > 350 && flipTo(stop + 1)}
              >
                <span aria-hidden="true" className={`${styles.ear} ${styles.earRight}`} style={{ opacity: stop === last - 1 ? 0 : 1 }} />
              </button>
            </>
          )}
        </div>
        {tgtStop !== 0 && tgtStop !== last && (
          <nav
            aria-label="Jump to a letter"
            className={styles.tabs}
            onPointerDown={(event) => {
              if (turn) {
                return
              }
              const t = tabAt(event.currentTarget, event.clientY)
              if (!t) {
                return
              }
              tabDrag.current = { nav: event.currentTarget, id: event.pointerId, start: t.i, cur: t.i, y0: event.clientY, moved: false, book }
              event.currentTarget.setPointerCapture?.(event.pointerId)
              event.preventDefault()
            }}
            onPointerMove={(event) => {
              const d = tabDrag.current
              if (d?.book !== book) {
                tabDrag.current = null
                return
              }
              if (event.pointerId !== d.id || (!d.moved && Math.abs(event.clientY - d.y0) < 6)) {
                return
              }
              const t = tabAt(d.nav, event.clientY)
              if (t && (!d.moved || t.i !== d.cur)) {
                d.moved = true
                d.cur = t.i
                setScrub({ tab: t.i, top: t.top, book })
                onAt(tabTargets[t.i]!)
              }
            }}
            onPointerUp={(event) => {
              const d = tabDrag.current
              if (!d || event.pointerId !== d.id) {
                return
              }
              tabDrag.current = null
              tabHandled.current = event.timeStamp
              if (d.moved || d.book !== book) {
                setScrub(null)
              } else {
                goTo(tabTargets[d.start]!)
              }
            }}
            onPointerCancel={() => {
              tabDrag.current = null
              setScrub(null)
            }}
          >
            {tabs.map((t, i) => (
              <button
                key={t.label}
                type="button"
                data-tab={i}
                aria-label={t.aria}
                aria-current={t.on ? 'true' : undefined}
                className={`${styles.tab} ${t.on ? styles.tabOn : ''} ${scrub?.tab === i ? styles.tabScrub : ''}`}
                // The pointer already handled it; a keyboard Enter or Space arrives only as this click.
                onClick={(event) => event.timeStamp - tabHandled.current > 400 && goTo(tabTargets[i]!)}
              >
                {t.label}
              </button>
            ))}
            {scrub && (
              <span aria-hidden="true" className={styles.scrub} style={{ top: scrub.top }}>
                {tabs[scrub.tab]!.label}
              </span>
            )}
          </nav>
        )}
      </div>
      <div className={styles.controls} data-controls>
        {/* Closed, the line under the cover says how to open it; the buttons and page box come with the open book. */}
        {!closed && (
          <>
            <button type="button" className={`pill ${styles.prevButton}`} onClick={() => flipTo(stop - 1)}>
              ◀ <span className={styles.word}>Previous</span>
            </button>
            <form
              className={styles.pageBox}
              onSubmit={(event) => {
                event.preventDefault()
                goToPage()
              }}
            >
              <label htmlFor={pageField} className={styles.word}>
                Page
              </label>
              <input
                id={pageField}
                className={styles.pageField}
                inputMode="numeric"
                autoComplete="off"
                enterKeyHint="go"
                size={3}
                value={draft ?? boxNumber}
                onFocus={(event) => event.currentTarget.select()}
                onChange={(event) => setDraft(event.currentTarget.value.replace(/\D/g, ''))}
                onKeyDown={(event) => event.key === 'Escape' && setDraft(null)}
                onBlur={goToPage}
              />
              <span>of {total}</span>
            </form>
          </>
        )}
        <span aria-live="polite" className={closed ? styles.position : 'visually-hidden'}>
          {position}
        </span>
        {!closed && (
          <button type="button" className={`pill ${styles.nextButton}`} onClick={() => flipTo(stop + 1)}>
            <span className={styles.word}>Next</span> ▶
          </button>
        )}
      </div>
    </div>
  )
}
