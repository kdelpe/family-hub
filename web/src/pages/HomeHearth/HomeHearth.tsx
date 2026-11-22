import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { m } from 'motion/react'
import { type ReactNode, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'

import { type Birthday, type Home, homeQuery, type Newcomer } from '../../api/home'
import { addedOn, inDays, monthAndDay } from '../../lib/dates'
import { dayPartLabel, greeting } from '../../lib/dayPart'
import { errorMessage } from '../../lib/errorMessage'
import { useMotionPaused, useReducedMotion } from '../../lib/motion'
import { fullName, initials } from '../../lib/names'
import { useRelations } from '../../lib/useRelations'
import { useSignedIn } from '../../lib/useSignedIn'
import { PersonHoverCard } from '../../components/PersonHoverCard/PersonHoverCard'
import styles from './HomeHearth.module.css'

// Home (design-notes §4.1): the hero, Coming up and New in the family. On this day and Story of the week wait for
// photos and stories (Phase 5).
export function HomeHearth({ now = new Date() }: { now?: Date }) {
  const { user, person } = useSignedIn()
  const hello = greeting(now)
  const home = useQuery(homeQuery)
  const next = home.data?.birthdays[0]

  return (
    <main className={styles.page}>
      <title>Home — Pluviose Family Hub</title>
      <section aria-label="Welcome" className={styles.hero}>
        <div className={styles.words}>
          <span className={styles.dayPart}>{dayPartLabel(now)}</span>
          <h1 className={styles.title}>{person ? `${hello}, ${person.first_name}` : hello}</h1>
          {next && <p className={styles.line}>{birthdayLine(next, user?.person_id ?? null)}</p>}
          <div className={styles.actions}>
            <Link to="/tree" className={`pill ${styles.primary}`}>
              Open the family tree
            </Link>
            <Link to="/events" className={`pill ${styles.secondary}`}>
              See new photos
            </Link>
          </div>
        </div>
        <HeroTree />
      </section>
      <div className={styles.cards}>
        <Card
          id="home-coming-up"
          title="Coming up"
          more="See all"
          className={styles.first}
          query={home}
          empty="No birthdays in the next two weeks."
          items={home.data?.birthdays}
        >
          {(b, i) => <BirthdayRow key={b.id} birthday={b} soonest={i === 0} />}
        </Card>
        <Newcomers query={home} viewerId={user?.person_id ?? null} now={now} />
      </div>
    </main>
  )
}

// The soonest birthday, said in the hero (owner's call, 2026-11-21).
function birthdayLine(b: Birthday, viewerId: string | null) {
  const whose = b.id === viewerId ? 'Your' : `${b.first_name}’s`
  const when = b.in_days === 0 ? 'today' : b.in_days === 1 ? 'tomorrow' : `in ${b.in_days} days`
  return `${whose} birthday is ${when}.`
}

const branches = ['M100 145V70', 'M100 100L60 60', 'M100 90l42-38', 'M100 75L78 35', 'M100 72l20-42']
const leaves = [
  { cx: 60, cy: 58, r: 13, light: false },
  { cx: 143, cy: 50, r: 15, light: false },
  { cx: 78, cy: 32, r: 12, light: false },
  { cx: 121, cy: 28, r: 13, light: false },
  { cx: 100, cy: 20, r: 11, light: false },
  { cx: 44, cy: 74, r: 8, light: true },
  { cx: 160, cy: 68, r: 9, light: true },
]
// It grows once per visit; coming back to Home shows it grown.
let grown = false

function HeroTree() {
  const [paused] = useMotionPaused()
  const reduced = useReducedMotion()
  const [grow] = useState(() => !grown && !reduced)
  useEffect(() => {
    grown = true
  }, [])
  const still = paused || reduced

  return (
    <svg aria-hidden="true" className={styles.tree} viewBox="0 0 200 150">
      {branches.map((d, i) => (
        <m.path
          key={d}
          d={d}
          className={styles.branches}
          initial={grow ? { pathLength: 0 } : false}
          animate={{ pathLength: 1 }}
          transition={{ duration: 0.9, delay: 0.15 + i * 0.12, ease: 'easeOut' }}
        />
      ))}
      {leaves.map((leaf, i) => (
        <m.circle
          key={i}
          cx={leaf.cx}
          cy={leaf.cy}
          r={leaf.r}
          className={leaf.light ? styles.lightLeaf : styles.leaf}
          initial={grow ? { scale: 0 } : false}
          animate={still ? { scale: 1, y: 0 } : { scale: 1, y: [0, -2.5, 0] }}
          transition={{
            scale: { type: 'spring', stiffness: 260, damping: 12, delay: grow ? 1 + i * 0.1 : 0 },
            y: still ? { duration: 0.3 } : { duration: 5 + (i % 3), repeat: Infinity, ease: 'easeInOut', delay: (grow ? 2 : 0) + i * 0.3 },
          }}
        />
      ))}
    </svg>
  )
}

const shownAtFirst = 3

// A card with its title, and the button that shows the rest of its rows beside it.
function Card<T>({
  id,
  title,
  more,
  className,
  query,
  items,
  empty,
  children,
}: {
  id: string
  title: string
  more: string
  className?: string
  query: UseQueryResult<Home>
  items: T[] | undefined
  empty: string
  children: (item: T, index: number) => ReactNode
}) {
  const [open, setOpen] = useState(false)
  const rows = useRef<HTMLDivElement>(null)
  const byKeyboard = useRef(false)
  // The button is gone once pressed, so keyboard focus moves on to the first row it showed.
  useEffect(() => {
    if (open && byKeyboard.current) {
      rows.current?.children[shownAtFirst]?.querySelector('a')?.focus()
    }
  }, [open])

  let body: ReactNode
  if (query.isError && !items) {
    body = <p role="alert">{errorMessage(query.error)}</p>
  } else if (!items) {
    body = <p className={styles.muted}>Loading…</p>
  } else if (items.length === 0) {
    body = <p className={styles.muted}>{empty}</p>
  } else {
    body = (
      <div ref={rows} className={styles.rows}>
        {(open ? items : items.slice(0, shownAtFirst)).map(children)}
      </div>
    )
  }
  return (
    <section aria-labelledby={id} className={`${styles.card} ${className ?? ''}`}>
      <div className={styles.cardHead}>
        <h2 id={id} className={styles.cardTitle}>
          {title}
        </h2>
        {!open && items && items.length > shownAtFirst && (
          <button
            type="button"
            onClick={(event) => {
              // detail is 0 for a keyboard press; a mouse click keeps focus where it is.
              byKeyboard.current = event.detail === 0
              setOpen(true)
            }}
            className={styles.more}
          >
            {more}
          </button>
        )}
      </div>
      {body}
    </section>
  )
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
        <PersonHoverCard person={birthday}>
          <Link to={`/people/${birthday.id}`} className={styles.rowTitle}>
            {birthday.first_name}’s birthday
          </Link>
        </PersonHoverCard>
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
  const { relations, isPending } = useRelations(viewerId, needsTree)
  // Held until the tree is read, so a row's line doesn't change under the reader; without it, just when.
  const ready = !needsTree || !isPending
  return (
    <Card
      id="home-new"
      title="New in the family"
      more="See more"
      className={styles.second}
      query={query}
      empty="Nobody new in the last month."
      items={ready ? newcomers : undefined}
    >
      {(n: Newcomer) => {
        const relation = relations?.get(n.id)
        const added = `added ${addedOn(n.added_at, now)}`
        return (
          <div key={n.id} className={styles.row}>
            <span aria-hidden="true" className={styles.avatar}>
              {initials(n)}
            </span>
            <div className={styles.rowWords}>
              <PersonHoverCard person={n}>
                <Link to={`/people/${n.id}`} className={styles.rowTitle}>
                  {fullName(n)}
                </Link>
              </PersonHoverCard>
              <span className={styles.muted}>{relation ? `${relation} · ${added}` : added[0]!.toUpperCase() + added.slice(1)}</span>
            </div>
          </div>
        )
      }}
    </Card>
  )
}
