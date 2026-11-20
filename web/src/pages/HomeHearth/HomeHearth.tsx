import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { type ReactNode, useMemo } from 'react'
import { Link } from 'react-router'

import { type Birthday, type Home, homeQuery, type Newcomer } from '../../api/home'
import { treeQuery } from '../../api/tree'
import { addedOn, inDays, monthAndDay } from '../../lib/dates'
import { dayPartLabel, greeting } from '../../lib/dayPart'
import { errorMessage } from '../../lib/errorMessage'
import { fullName, initials } from '../../lib/names'
import { relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './HomeHearth.module.css'

// Home (design-notes §4.1): the hero's words and buttons, Coming up and New in the family. The hero's personal
// line, its small tree, On this day and Story of the week wait for photos and stories (Phase 5).
export function HomeHearth({ now = new Date() }: { now?: Date }) {
  const { user, person } = useSignedIn()
  const hello = greeting(now)
  const home = useQuery(homeQuery)

  return (
    <main className={styles.page}>
      <title>Home — Pluviose-Monestime Family Hub</title>
      <section aria-label="Welcome" className={styles.hero}>
        <div className={styles.words}>
          <span className={styles.dayPart}>{dayPartLabel(now)}</span>
          <h1 className={styles.title}>{person ? `${hello}, ${person.first_name}` : hello}</h1>
          <div className={styles.actions}>
            <Link to="/tree" className={`pill ${styles.primary}`}>
              Open the family tree
            </Link>
            <Link to="/events" className={`pill ${styles.secondary}`}>
              See new photos
            </Link>
          </div>
        </div>
      </section>
      <div className={styles.cards}>
        <section aria-labelledby="home-coming-up" className={`${styles.card} ${styles.first}`}>
          <h2 id="home-coming-up" className={styles.cardTitle}>
            Coming up
          </h2>
          <Cards query={home} empty="No birthdays in the next two weeks." items={home.data?.birthdays}>
            {(b, i) => <BirthdayRow key={b.id} birthday={b} soonest={i === 0} />}
          </Cards>
        </section>
        <section aria-labelledby="home-new" className={`${styles.card} ${styles.second}`}>
          <h2 id="home-new" className={styles.cardTitle}>
            New in the family
          </h2>
          <Newcomers query={home} viewerId={user?.person_id ?? null} now={now} />
          <Link to="/people" className={styles.seeAll}>
            See everyone
          </Link>
        </section>
      </div>
    </main>
  )
}

function Cards<T>({
  query,
  items,
  empty,
  children,
}: {
  query: UseQueryResult<Home>
  items: T[] | undefined
  empty: string
  children: (item: T, index: number) => ReactNode
}) {
  if (query.isError && !items) {
    return <p role="alert">{errorMessage(query.error)}</p>
  }
  if (!items) {
    return <p className={styles.muted}>Loading…</p>
  }
  return items.length === 0 ? <p className={styles.muted}>{empty}</p> : items.map(children)
}

function BirthdayRow({ birthday, soonest }: { birthday: Birthday; soonest: boolean }) {
  const { month, day } = monthAndDay(birthday.on)
  return (
    <div className={styles.row}>
      <span aria-hidden="true" className={soonest ? `${styles.tile} ${styles.soonest}` : styles.tile}>
        <span className={styles.tileMonth}>{month}</span>
        <span className={styles.tileDay}>{day}</span>
      </span>
      <div className={styles.rowWords}>
        <Link to={`/people/${birthday.id}`} className={styles.rowTitle}>
          {birthday.first_name}’s birthday
        </Link>
        <span className={styles.muted}>
          {inDays(birthday.in_days)}
          <span className="visually-hidden">, {month} {day}</span>
        </span>
      </div>
    </div>
  )
}

function Newcomers({ query, viewerId, now }: { query: UseQueryResult<Home>; viewerId: string | null; now: Date }) {
  const newcomers = query.data?.newcomers
  // The whole tree, only when there's someone to name a relation for.
  const needsTree = viewerId !== null && (newcomers?.length ?? 0) > 0
  const tree = useQuery({ ...treeQuery, enabled: needsTree })
  const relations = useMemo(() => (tree.data && viewerId ? relationsTo(tree.data, viewerId) : null), [tree.data, viewerId])
  // Held until the tree is read, so a row's line doesn't change under the reader; without it, just when.
  const ready = !needsTree || !tree.isPending
  return (
    <Cards query={query} empty="Nobody new in the last month." items={ready ? newcomers : undefined}>
      {(n: Newcomer) => {
        const relation = relations?.get(n.id)
        const added = `added ${addedOn(n.added_at, now)}`
        return (
          <div key={n.id} className={styles.row}>
            <span aria-hidden="true" className={styles.avatar}>
              {initials(n)}
            </span>
            <div className={styles.rowWords}>
              <Link to={`/people/${n.id}`} className={styles.rowTitle}>
                {fullName(n)}
              </Link>
              <span className={styles.muted}>{relation ? `${relation} · ${added}` : added[0]!.toUpperCase() + added.slice(1)}</span>
            </div>
          </div>
        )
      }}
    </Cards>
  )
}
