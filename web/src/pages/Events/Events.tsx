import { useIsMutating, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, m } from 'motion/react'
import { useEffect, useMemo, useRef, useState } from 'react'

import { onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { ApiError } from '../../api/client'
import { createEventKey, deleteEvent, eventKeys, eventsQuery, type FamilyEvent } from '../../api/events'
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog'
import { useToast } from '../../components/Toaster/Toaster'
import { bornOn, year } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { useMotionPaused, useReducedMotion } from '../../lib/motion'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './Events.module.css'
import { NewEvent } from './NewEvent'
import { hideEvent, unhideEvent, usePendingRemovals } from './pendingRemovals'
import { undated, YearHandle } from './YearHandle'

const yearOf = (event: FamilyEvent) => event.date?.match(/^\d{4}/)?.[0] ?? undated
const anchorId = (year: string) => `events-year-${year}`
// The handle's middle, down the album page: matches .handleColumn in Events.module.css.
const handleAt = 0.42
// From an entry's top to its dot's middle, which crosses the handle; measured, as it sits higher on phones.
const dotDown = (entry: HTMLElement) => {
  const dot = entry.querySelector<HTMLElement>('[data-dot]')!.getBoundingClientRect()
  return dot.top + dot.height / 2 - entry.getBoundingClientRect().top
}
const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
// The handle shows the month of the event it's on; with no month known, its year, or — when undated.
const handleLabel = (event: FamilyEvent | undefined) => {
  const month = event?.date?.match(/^\d{4}-(\d{2})/)?.[1]
  if (month) {
    return months[Number(month) - 1]!
  }
  const y = event ? yearOf(event) : undated
  return y === undated ? '—' : `’${y.slice(2)}`
}

// Events (design-notes §4.5): the timeline. Media, + Upload and each card's photos wait for Phase 5.
export function Events() {
  const events = useQuery(eventsQuery)
  const { user } = useSignedIn()
  const [adding, setAdding] = useState(false)
  // A fresh panel each time: reopened while the last one is still closing, it must not bring that one back.
  const [opened, setOpened] = useState(0)
  const saving = useIsMutating({ mutationKey: createEventKey }) > 0
  const [saved, setSaved] = useState<string | null>(null)
  const newButton = useRef<HTMLButtonElement>(null)
  const close = () => {
    setAdding(false)
    requestAnimationFrame(() => newButton.current?.focus())
  }

  const queryClient = useQueryClient()
  const toast = useToast()
  const [removing, setRemoving] = useState<FamilyEvent | null>(null)
  const [asking, setAsking] = useState(false)
  // Its × is gone once removed, so focus goes to + New event instead.
  const removed = useRef(false)
  const hidden = usePendingRemovals()
  const [onEvent, setOnEvent] = useState<{ id: string; year: string } | null>(null)
  // A picked year holds until the reader scrolls or presses something themselves, so the glide's own scroll events
  // don't move it back.
  const held = useRef(false)
  const list = useMemo(() => (events.data ?? []).filter((e) => !hidden.has(e.id)), [events.data, hidden])
  const years = useMemo(() => [...new Set(list.map(yearOf))], [list])
  // The event the handle is on, and its year, which holds if a refresh drops that event.
  const shown = list.find((e) => e.id === onEvent?.id) ?? list.find((e) => yearOf(e) === onEvent?.year) ?? list[0]
  // It leaves the timeline at once; the server is only told once Undo has run out.
  const remove = (event: FamilyEvent) => {
    // Signed out or in as someone else during Undo: nothing is sent, and no answer is shown to them.
    const since = sessionGeneration()
    // The handle moves on to the next event along, where the reader is.
    // Held there, as the timeline closes up under it without a scroll.
    if (shown?.id === event.id) {
      const i = list.findIndex((e) => e.id === event.id)
      const next = list[i + 1] ?? list[i - 1]
      setOnEvent(next ? { id: next.id, year: yearOf(next) } : null)
      held.current = true
    }
    hideEvent(event.id)
    toast({
      text: `Removed “${event.title}”.`,
      undo: () => unhideEvent(event.id),
      onDone: () => {
        if (!sameSession(since)) {
          unhideEvent(event.id)
          return
        }
        void deleteEvent(event.id)
          .then(
            () => true,
            (error: unknown) => {
              if (error instanceof ApiError && error.status === 404) {
                return true
              }
              // A 401 means the session ended on the server: what was fetched as them goes, as for any save.
              if (error instanceof Error) {
                onSessionEnded(queryClient)(error, undefined, since)
              }
              if (!sameSession(since)) {
                return false
              }
              toast({ text: `Couldn’t remove “${event.title}”. ${error instanceof Error ? errorMessage(error) : ''}`.trimEnd() })
              return false
            },
          )
          .then((gone) => {
            if (gone) {
              queryClient.setQueryData<FamilyEvent[]>(eventKeys.all, (all) => all?.filter((e) => e.id !== event.id))
            }
            unhideEvent(event.id)
            if (sameSession(since)) {
              void queryClient.invalidateQueries({ queryKey: eventKeys.all })
            }
          })
      },
    })
  }

  const current = shown ? yearOf(shown) : undated
  const reduced = useReducedMotion()
  const [motionPaused] = useMotionPaused()
  const album = useRef<HTMLDivElement>(null)
  // State, not a ref: a failed refetch swaps the timeline for its error, and the one that comes back needs following.
  const [scroller, setScroller] = useState<HTMLDivElement | null>(null)
  useEffect(() => {
    const box = scroller
    if (years.length < 2 || !box) {
      return
    }
    const shownIds = new Set(list.map((e) => e.id))
    const follow = () => {
      if (held.current) {
        return
      }
      // Not one sliding out after its removal.
      const entries = [...box.querySelectorAll<HTMLElement>('[data-event]')].filter((el) => shownIds.has(el.dataset.event!))
      // The very top is the newest and the very bottom the oldest, so every year can be reached however few there are.
      let at = entries[0]
      if (box.scrollTop > 1 && box.scrollTop + box.clientHeight >= box.scrollHeight - 1) {
        at = entries.at(-1)
      } else if (box.scrollTop > 1) {
        const r = album.current!.getBoundingClientRect()
        const line = r.top + r.height * handleAt
        for (const el of entries) {
          if (el.getBoundingClientRect().top + dotDown(el) > line + 1) {
            break
          }
          at = el
        }
      }
      const id = at?.dataset.event
      const found = list.find((e) => e.id === id)
      // The same event keeps the same state, so a scroll within it renders nothing.
      setOnEvent((was) => (was?.id === found?.id ? was : found ? { id: found.id, year: yearOf(found) } : null))
    }
    // Capture, so a press on the handle releases first and its own pick then holds again.
    const release = () => {
      held.current = false
    }
    const first = requestAnimationFrame(follow)
    const passive = { passive: true }
    const capture = { capture: true, passive: true }
    box.addEventListener('scroll', follow, passive)
    window.addEventListener('wheel', release, passive)
    window.addEventListener('touchstart', release, passive)
    window.addEventListener('pointerdown', release, capture)
    window.addEventListener('keydown', release, capture)
    return () => {
      cancelAnimationFrame(first)
      box.removeEventListener('scroll', follow)
      window.removeEventListener('wheel', release)
      window.removeEventListener('touchstart', release)
      window.removeEventListener('pointerdown', release, capture)
      window.removeEventListener('keydown', release, capture)
    }
  }, [list, years, scroller])
  const pick = (year: string, glide: boolean) => {
    const i = years.indexOf(year)
    // The oldest is at the very bottom, where following the scroll reads the last event.
    const on = i === years.length - 1 ? list.at(-1) : list.find((e) => yearOf(e) === year)
    const box = scroller
    // By its event, not its year's anchor, which one sliding out after its removal may still carry too.
    const el = on && box?.querySelector<HTMLElement>(`[data-event="${on.id}"]`)
    if (!on || !el || !box || !album.current) {
      return
    }
    setOnEvent({ id: on.id, year })
    held.current = true
    const line = album.current.clientHeight * handleAt - (box.getBoundingClientRect().top - album.current.getBoundingClientRect().top)
    const top =
      i === 0 ? 0 : i === years.length - 1 ? box.scrollHeight : el.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop - line + dotDown(el)
    box.scrollTo({ top, behavior: glide && !reduced ? 'smooth' : 'auto' })
  }

  return (
    <main className={styles.page}>
      <title>Events — Pluviose Family Hub</title>
      <div className={styles.head}>
        <h1 className={styles.title}>Events</h1>
        {user?.role === 'admin' && (
          <button
            ref={newButton}
            type="button"
            aria-expanded={adding}
            disabled={saving}
            onClick={() => {
              setSaved(null)
              setOpened((n) => (adding ? n : n + 1))
              setAdding((open) => !open)
            }}
            className={`pill ${styles.newEvent}`}
          >
            + New event
          </button>
        )}
      </div>
      {/* One panel at a time: the next mounts once the last has gone, so their field ids never meet. */}
      <AnimatePresence mode="wait" initial={false}>
        {adding && (
          <m.div
            key={opened}
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            // A height change isn't a transform, so MotionConfig doesn't skip it for reduced motion or Pause.
            transition={reduced || motionPaused ? { duration: 0 } : { type: 'spring', stiffness: 300, damping: 32 }}
            className={styles.panelOpening}
          >
            <NewEvent
              onClose={close}
              onSaved={(title) => {
                setSaved(title)
                close()
              }}
            />
          </m.div>
        )}
      </AnimatePresence>
      <p role="status" className={saved ? styles.saved : 'visually-hidden'}>
        {saved && `Saved “${saved}”.`}
      </p>
      {events.isPending ? (
        <p className={styles.muted}>Loading…</p>
      ) : events.isError ? (
        <p role="alert">{errorMessage(events.error)}</p>
      ) : list.length === 0 ? (
        <p className={styles.muted}>No events yet.</p>
      ) : (
        <div ref={album} className={styles.album}>
          {/* Focusable, so the timeline scrolls from the keyboard too. */}
          {/* layoutScroll: the glides inside measure past how far it's scrolled. */}
          <m.div ref={setScroller} layoutScroll role="region" aria-label="Timeline" tabIndex={0} className={styles.scroller}>
            <ol className={styles.timeline}>
            <AnimatePresence initial={false}>
              {list.map((event, i) => {
                const y = yearOf(event)
                const first = i === 0 || yearOf(list[i - 1]!) !== y
                return (
                  <TimelineEntry
                    key={event.id}
                    event={event}
                    anchor={first ? anchorId(y) : undefined}
                    side={i % 2 === 0 ? 'left' : 'right'}
                    first={i === 0}
                    last={i === list.length - 1}
                    lit={years.length > 1 && y === current}
                    onRemove={
                      user?.role === 'admin'
                        ? () => {
                            removed.current = false
                            setRemoving(event)
                            setAsking(true)
                          }
                        : undefined
                    }
                  />
                )
              })}
            </AnimatePresence>
            </ol>
          </m.div>
          {years.length > 1 && (
            <YearHandle years={years} current={current} label={handleLabel(shown)} still={reduced} onPick={pick} />
          )}
        </div>
      )}
      <ConfirmDialog
        open={asking}
        question={`Remove “${removing?.title ?? ''}”?`}
        note="It leaves the timeline for everyone. You’ll have a few seconds to undo."
        yes="Remove"
        no="Keep it"
        busy={false}
        onYes={() => {
          removed.current = true
          remove(removing!)
          setAsking(false)
        }}
        onCancel={() => setAsking(false)}
        focusAfterClose={() => (removed.current ? newButton.current : null)}
      />
    </main>
  )
}

function TimelineEntry({
  event,
  anchor,
  side,
  first,
  last,
  lit,
  onRemove,
}: {
  event: FamilyEvent
  anchor?: string
  side: 'left' | 'right'
  // Classes, not :first-child: a removed entry stays in the list while it slides out.
  first: boolean
  last: boolean
  lit: boolean
  onRemove?: () => void
}) {
  const when = whenItHappened(event)
  return (
    <m.li
      id={anchor}
      data-event={event.id}
      layout="position"
      initial={{ opacity: 0, y: -14 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, x: 40, transition: { duration: 0.2 } }}
      transition={{ type: 'spring', stiffness: 380, damping: 32 }}
      className={[styles.entry, side === 'left' ? styles.entryLeft : styles.entryRight, first && styles.first, last && styles.last].filter(Boolean).join(' ')}
    >
      {/* Each glides to its new side when an event added or removed above swaps the sides. */}
      <m.span layout="position" className={styles.year}>
        {/* Lit on a span of its own: Motion's layout glide leaves its own transform-origin on the outer one. */}
        <span className={lit ? `${styles.yearText} ${styles.yearLit}` : styles.yearText}>{year(event.date, event.date_precision)}</span>
      </m.span>
      <span aria-hidden="true" className={styles.line}>
        <span data-dot className={styles.dot} />
      </span>
      <span aria-hidden="true" className={styles.tick} />
      <m.article layout="position" className={styles.card}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}>{event.title}</h2>
          {onRemove && (
            <button type="button" aria-label={`Remove “${event.title}”`} onClick={onRemove} className={styles.removeEvent}>
              ×
            </button>
          )}
          {(when || event.location) && <span className={styles.when}>{[when, event.location].filter(Boolean).join(' · ')}</span>}
        </div>
        {event.description && <p className={styles.story}>{event.description}</p>}
      </m.article>
    </m.li>
  )
}

// "November 13, 1984", "July 2025", "1984", or "Around 1962", as the design writes them.
function whenItHappened(event: FamilyEvent) {
  if (event.date_precision === 'circa') {
    return `Around ${event.date}`
  }
  return bornOn(event.date, event.date_precision)
}
