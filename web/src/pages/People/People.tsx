import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'

import { familySummaryQuery } from '../../api/family'
import { type TreePerson, treeQuery } from '../../api/tree'
import { FamilyStrip } from '../../components/FamilyStrip/FamilyStrip'
import { PhotoFade } from '../../components/PhotoFade/PhotoFade'
import { errorMessage } from '../../lib/errorMessage'
import { sideLabel, sideTags } from '../../lib/familySides'
import { useMotionPaused, useReducedMotion } from '../../lib/motion'
import { fullName, initials, nameMatches } from '../../lib/names'
import { relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './People.module.css'

type Filter = 'everyone' | 'living' | 'remembered' | 'first' | 'second'

const filters: { id: Filter; label: string }[] = [
  { id: 'everyone', label: 'Everyone' },
  { id: 'living', label: 'Living' },
  { id: 'remembered', label: 'Remembered' },
  { id: 'first', label: sideLabel('first') },
  { id: 'second', label: sideLabel('second') },
]

// Drawing thousands of cards at once stalls a phone, so the grid grows a page at a time.
const pageSize = 60

const bands = { first: styles.bandFirst, second: styles.bandSecond, both: styles.bandBoth, married_in: styles.bandMarriedIn }

// Someone on both sides belongs to either side's filter.
const keep: Record<Filter, (p: TreePerson) => boolean> = {
  everyone: () => true,
  living: (p) => !p.is_deceased,
  remembered: (p) => p.is_deceased,
  first: (p) => p.side === 'first' || p.side === 'both',
  second: (p) => p.side === 'second' || p.side === 'both',
}

// People (design-notes §4.3): everyone in the archive, living or not, with or without a login.
export function People() {
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const tree = useQuery(treeQuery)
  const summary = useQuery(familySummaryQuery)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('everyone')
  const [shown, setShown] = useState(pageSize)
  const grid = useRef<HTMLUListElement>(null)
  const firstNew = useRef<number | null>(null)
  // After a keyboard press, focus moves to the first card it brought in.
  useEffect(() => {
    if (firstNew.current !== null) {
      grid.current?.children[firstNew.current]?.querySelector('a')?.focus()
      firstNew.current = null
    }
  }, [shown])

  const [paused, setPaused] = useMotionPaused()
  const reducedMotion = useReducedMotion()
  const relations = useMemo(
    () => (tree.data && viewerId ? relationsTo(tree.data, viewerId) : new Map<string, string>()),
    [tree.data, viewerId],
  )
  const people = useMemo(
    () =>
      (tree.data?.persons ?? [])
        .map((p) => ({ p, name: fullName(p) }))
        .filter(({ p, name }) => keep[filter](p) && nameMatches(name, query))
        // A–Z by first name (design-notes §4.3), then the rest of the name.
        .sort((a, b) => a.p.first_name.localeCompare(b.p.first_name) || a.name.localeCompare(b.name))
        .map(({ p }) => p),
    [tree.data, filter, query],
  )

  return (
    <main className={styles.page}>
      <title>People — Pluviose-Monestime Family Hub</title>
      <div className={styles.top}>
        <h1 className={styles.title}>People</h1>
        <label htmlFor="pl-search" className="visually-hidden">
          Search people
        </label>
        <input
          id="pl-search"
          type="search"
          placeholder="Search by name…"
          autoComplete="off"
          value={query}
          onChange={(event) => {
            setQuery(event.target.value)
            setShown(pageSize)
          }}
          className={styles.search}
        />
      </div>

      {summary.data && <FamilyStrip summary={summary.data} />}

      <div className={styles.filterRow}>
        <div role="group" aria-label="Filter people" className={styles.filters}>
          {filters.map((f) => (
            <button
              key={f.id}
              type="button"
              aria-pressed={filter === f.id}
              className={filter === f.id ? `pill ${styles.filter} ${styles.filterOn}` : `pill ${styles.filter}`}
              onClick={() => {
                setFilter(f.id)
                setShown(pageSize)
              }}
            >
              {f.label}
            </button>
          ))}
        </div>
        {/* The portraits' fade runs on; with reduced motion there's none to pause (design-notes §6.4). */}
        {!reducedMotion && (
          <button type="button" className={styles.pause} onClick={() => setPaused(!paused)}>
            {paused ? 'Play motion' : 'Pause motion'}
          </button>
        )}
      </div>

      {tree.error && <p role="alert">{errorMessage(tree.error)}</p>}
      {tree.data && people.length === 0 && (
        <p className={styles.none}>
          {tree.data.persons.length === 0 ? 'Nobody is in the family tree yet.' : 'Nobody here matches.'}
        </p>
      )}
      {people.length > 0 && (
        <ul ref={grid} className={styles.grid}>
          {people.slice(0, shown).map((p) => (
            <li key={p.id}>
              <Card person={p} relation={relations.get(p.id) ?? null} you={p.id === viewerId} paused={paused} />
            </li>
          ))}
        </ul>
      )}
      {people.length > shown && (
        <button
          type="button"
          onClick={(event) => {
            // detail is 0 for a keyboard press; a mouse click keeps focus, and the page, where they are.
            firstNew.current = event.detail === 0 ? shown : null
            setShown((n) => n + pageSize)
          }}
          className={`pill ${styles.filter} ${styles.more}`}
        >
          Show more
        </button>
      )}
    </main>
  )
}

function Card({ person, relation, you, paused }: { person: TreePerson; relation: string | null; you: boolean; paused: boolean }) {
  const side = person.side ? { label: sideTags[person.side], band: bands[person.side] } : null
  return (
    <Link to={`/people/${person.id}`} className={you ? `${styles.card} ${styles.cardYou}` : styles.card}>
      <span aria-hidden="true" className={`${styles.band} ${you ? styles.bandYou : (side?.band ?? '')}`}>
        <svg viewBox="0 0 200 64" preserveAspectRatio="none" className={styles.mountain}>
          <path d="M0 58 Q26 30 54 42 Q88 12 120 36 Q152 20 200 48" vectorEffect="non-scaling-stroke" />
        </svg>
      </span>
      <span aria-hidden="true" className={you ? `${styles.portrait} ${styles.portraitYou}` : styles.portrait}>
        {initials(person)}
        <PhotoFade person={person} paused={paused} />
      </span>
      <span className={styles.words}>
        <span className={styles.name}>{fullName(person)}</span>
        {relation && <span className={styles.relation}>{relation}</span>}
        <span className={styles.tags}>
          {side && <span className={`${styles.tag} ${you ? styles.bandYou : side.band}`}>{side.label}</span>}
          {person.is_deceased && (
            <span className={`${styles.tag} ${styles.memory}`}>
              <svg aria-hidden="true" width="10" height="12" viewBox="0 0 10 12">
                <path d="M5 11 C1 8 1 3 5 1 C9 3 9 8 5 11 Z" />
              </svg>
              In memory
            </span>
          )}
        </span>
      </span>
    </Link>
  )
}
