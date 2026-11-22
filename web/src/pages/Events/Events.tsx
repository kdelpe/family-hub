import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

import { eventsQuery, type FamilyEvent } from '../../api/events'
import { bornOn, year } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { fullName } from '../../lib/names'
import styles from './Events.module.css'

// Events (design-notes §4.5): the timeline. Media, + Upload and each card's photos wait for Phase 5.
export function Events() {
  const events = useQuery(eventsQuery)

  return (
    <main className={styles.page}>
      <title>Events — Pluviose Family Hub</title>
      <div className={styles.head}>
        <h1 className={styles.title}>Events</h1>
      </div>
      {events.isPending ? (
        <p className={styles.muted}>Loading…</p>
      ) : events.isError ? (
        <p role="alert">{errorMessage(events.error)}</p>
      ) : events.data.length === 0 ? (
        <p className={styles.muted}>No events yet.</p>
      ) : (
        <ol className={styles.timeline}>
          {events.data.map((event) => (
            <TimelineEntry key={event.id} event={event} />
          ))}
        </ol>
      )}
    </main>
  )
}

function TimelineEntry({ event }: { event: FamilyEvent }) {
  const when = whenItHappened(event)
  return (
    <li className={styles.entry}>
      <span className={styles.year}>{year(event.date, event.date_precision)}</span>
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
