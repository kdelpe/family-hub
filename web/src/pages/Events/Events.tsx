import { useIsMutating, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'

import { createEventKey, eventsQuery, type FamilyEvent } from '../../api/events'
import { bornOn, year } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { useReducedMotion } from '../../lib/motion'
import { fullName } from '../../lib/names'
import { useNarrow } from '../../lib/useNarrow'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './Events.module.css'
import { NewEvent } from './NewEvent'
import { undated, YearHandle } from './YearHandle'

const yearOf = (event: FamilyEvent) => event.date?.match(/^\d{4}/)?.[0] ?? undated
const anchorId = (year: string) => `events-year-${year}`
// Matches the handle's sticky top in Events.module.css.
const handleTop = () => window.innerHeight * 0.42
const handleMiddle = 58

// Events (design-notes §4.5): the timeline. Media, + Upload and each card's photos wait for Phase 5.
export function Events() {
  const events = useQuery(eventsQuery)
  const { user } = useSignedIn()
  const [adding, setAdding] = useState(false)
  const saving = useIsMutating({ mutationKey: createEventKey }) > 0
  const [saved, setSaved] = useState<string | null>(null)
  const newButton = useRef<HTMLButtonElement>(null)
  const close = () => {
    setAdding(false)
    requestAnimationFrame(() => newButton.current?.focus())
  }

  const list = events.data
  const years = useMemo(() => [...new Set((list ?? []).map(yearOf))], [list])
  const [picked, setPicked] = useState<string | null>(null)
  const current = picked && years.includes(picked) ? picked : (years[0] ?? undated)
  const reduced = useReducedMotion()
  const narrow = useNarrow()
  // Keeps a glide's own scroll events from moving the handle back.
  const holdUntil = useRef(0)
  useEffect(() => {
    if (years.length < 2) {
      return
    }
    const follow = () => {
      if (Date.now() < holdUntil.current) {
        return
      }
      const line = handleTop() + handleMiddle
      let at = years[0]!
      for (const y of years) {
        const el = document.getElementById(anchorId(y))
        if (el && el.getBoundingClientRect().top <= line) {
          at = y
        }
      }
      setPicked(at)
    }
    window.addEventListener('scroll', follow, { passive: true })
    return () => window.removeEventListener('scroll', follow)
  }, [years])
  const pick = (year: string, glide: boolean) => {
    const el = document.getElementById(anchorId(year))
    if (!el) {
      return
    }
    const smooth = glide && !reduced
    setPicked(year)
    holdUntil.current = Date.now() + (smooth ? 700 : 100)
    // On phones the year sits above its card, so it stops above the handle.
    const offset = handleTop() - (narrow ? 40 : 6)
    window.scrollTo({ top: el.getBoundingClientRect().top + window.scrollY - offset, behavior: smooth ? 'smooth' : 'auto' })
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
              setAdding((open) => !open)
            }}
            className={`pill ${styles.newEvent}`}
          >
            + New event
          </button>
        )}
      </div>
      {adding && (
        <NewEvent
          onClose={close}
          onSaved={(title) => {
            setSaved(title)
            close()
          }}
        />
      )}
      <p role="status" className={saved ? styles.saved : 'visually-hidden'}>
        {saved && `Saved “${saved}”.`}
      </p>
      {events.isPending ? (
        <p className={styles.muted}>Loading…</p>
      ) : events.isError ? (
        <p role="alert">{errorMessage(events.error)}</p>
      ) : events.data.length === 0 ? (
        <p className={styles.muted}>No events yet.</p>
      ) : (
        <div className={styles.timelineWrap}>
          <ol className={styles.timeline}>
            {events.data.map((event, i) => {
              const y = yearOf(event)
              const first = i === 0 || yearOf(events.data[i - 1]!) !== y
              return <TimelineEntry key={event.id} event={event} anchor={first ? anchorId(y) : undefined} lit={years.length > 1 && y === current} />
            })}
          </ol>
          {years.length > 1 && <YearHandle years={years} current={current} onPick={pick} />}
        </div>
      )}
    </main>
  )
}

function TimelineEntry({ event, anchor, lit }: { event: FamilyEvent; anchor?: string; lit: boolean }) {
  const when = whenItHappened(event)
  return (
    <li id={anchor} className={styles.entry}>
      <span className={lit ? `${styles.year} ${styles.yearLit}` : styles.year}>{year(event.date, event.date_precision)}</span>
      <span aria-hidden="true" className={styles.line}>
        <span className={styles.dot} />
      </span>
      <article className={styles.card}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}>{event.title}</h2>
          {(when || event.location) && <span className={styles.when}>{[when, event.location].filter(Boolean).join(' · ')}</span>}
        </div>
        {event.description && <p className={styles.story}>{event.description}</p>}
        {event.people.length > 0 && (
          <ul aria-label="Who was there" className={styles.people}>
            {event.people.map((p) => (
              <li key={p.id}>
                <Link to={`/people/${p.id}`} className={styles.chip}>
                  {fullName(p)}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </article>
    </li>
  )
}

// "November 13, 1984", "July 2025", "1984", or "Around 1962", as the design writes them.
function whenItHappened(event: FamilyEvent) {
  if (event.date_precision === 'circa') {
    return `Around ${event.date}`
  }
  return bornOn(event.date, event.date_precision)
}
