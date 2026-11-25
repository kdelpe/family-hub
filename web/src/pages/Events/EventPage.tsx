import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router'

import { eventsQuery, type FamilyEvent } from '../../api/events'
import { year } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { whenItHappened } from './whenItHappened'
import styles from './EventPage.module.css'

// One event's own page (design-notes §4.5a, owner's pick B2, 2026-11-25): a full-width cover, then the story. Its
// photos and videos, and the viewer that opens them, come with uploads (Phase 5).
export function EventPage() {
  const { id = '' } = useParams()
  // The timeline's list: usually already loaded, and it gives the events either side.
  const events = useQuery(eventsQuery)
  const list = events.data ?? []
  const at = list.findIndex((e) => e.id === id)
  const event = list[at]

  return (
    <main className={styles.page}>
      <title>{`${event?.title ?? 'Event'} — Pluviose Family Hub`}</title>
      <Link to="/events" className={styles.back}>
        ‹ Events
      </Link>
      {events.isPending ? (
        <p className={styles.muted}>Loading…</p>
      ) : events.error && !events.data ? (
        <p role="alert">{errorMessage(events.error)}</p>
      ) : !event ? (
        <p className={styles.muted}>This event isn’t in the album. It may have been removed.</p>
      ) : (
        <>
          <article className={styles.album}>
            <div className={styles.cover}>
              <p className={styles.coverNote}>
                <b>No photos or videos yet</b>
                The first photo added becomes the cover.
              </p>
            </div>
            <div className={styles.body}>
              <h1 className={styles.title}>{event.title}</h1>
              <When event={event} />
              {event.description && <p className={styles.story}>{event.description}</p>}
            </div>
          </article>
          <nav aria-label="Other events" className={styles.nav}>
            <Neighbor event={list[at - 1]} label="‹ Newer" />
            <Neighbor event={list[at + 1]} label="Older ›" older />
          </nav>
        </>
      )}
    </main>
  )
}

function When({ event }: { event: FamilyEvent }) {
  const when = whenItHappened(event)
  return (when || event.location) && <p className={styles.when}>{[when, event.location].filter(Boolean).join(' · ')}</p>
}

function Neighbor({ event, label, older = false }: { event?: FamilyEvent; label: string; older?: boolean }) {
  if (!event) {
    return <span />
  }
  const y = year(event.date, event.date_precision)
  return (
    <Link to={`/events/${event.id}`} className={older ? `${styles.neighbor} ${styles.older}` : styles.neighbor}>
      <small>{label}</small>
      <span>{y ? `${event.title} · ${y}` : event.title}</span>
    </Link>
  )
}
