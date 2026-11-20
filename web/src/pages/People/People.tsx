import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'

import { familyKeys, type FamilySummary, getFamilySummary } from '../../api/family'
import { getTree, type TreePerson, treeKeys } from '../../api/tree'
import { errorMessage } from '../../lib/errorMessage'
import { familyLines } from '../../lib/familySides'
import { fullName, initials, nameMatches } from '../../lib/names'
import { relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './People.module.css'

type Filter = 'everyone' | 'living' | 'remembered' | 'first' | 'second'

const filters: { id: Filter; label: string }[] = [
  { id: 'everyone', label: 'Everyone' },
  { id: 'living', label: 'Living' },
  { id: 'remembered', label: 'Remembered' },
  { id: 'first', label: `${familyLines.first} side` },
  { id: 'second', label: `${familyLines.second} side` },
]

const sides = {
  first: { label: `${familyLines.first} side`, band: styles.bandFirst },
  second: { label: `${familyLines.second} side`, band: styles.bandSecond },
  both: { label: 'Both sides', band: styles.bandBoth },
  married_in: { label: 'Married in', band: styles.bandMarriedIn },
}

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
  const tree = useQuery({ queryKey: treeKeys.all, queryFn: ({ signal }) => getTree({ signal }) })
  const summary = useQuery({ queryKey: familyKeys.summary, queryFn: ({ signal }) => getFamilySummary({ signal }) })
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('everyone')

  const relation = useMemo(
    () => (tree.data && viewerId ? relationsTo(tree.data, viewerId) : () => null),
    [tree.data, viewerId],
  )
  const people = (tree.data?.persons ?? [])
    .filter((p) => keep[filter](p) && nameMatches(fullName(p), query))
    .sort((a, b) => fullName(a).localeCompare(fullName(b)))

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
          onChange={(event) => setQuery(event.target.value)}
          className={styles.search}
        />
      </div>

      {summary.data && <Strip summary={summary.data} />}

      <div role="group" aria-label="Filter people" className={styles.filters}>
        {filters.map((f) => (
          <button
            key={f.id}
            type="button"
            aria-pressed={filter === f.id}
            className={filter === f.id ? `pill ${styles.filter} ${styles.filterOn}` : `pill ${styles.filter}`}
            onClick={() => setFilter(f.id)}
          >
            {f.label}
          </button>
        ))}
      </div>

      {tree.error && <p role="alert">{errorMessage(tree.error)}</p>}
      {tree.data && people.length === 0 && <p className={styles.none}>Nobody here matches.</p>}
      <ul className={styles.grid}>
        {people.map((p) => (
          <li key={p.id}>
            <Card person={p} relation={relation(p.id)} you={p.id === viewerId} />
          </li>
        ))}
      </ul>
    </main>
  )
}

function Strip({ summary }: { summary: FamilySummary }) {
  const earliest = summary.earliest_record
  return (
    <div className={styles.strip}>
      <span className={`${styles.stat} ${styles.statFirst}`}>
        <strong>{summary.people}</strong> {summary.people === 1 ? 'person' : 'people'}
      </span>
      <span className={styles.stat}>
        <strong>{summary.generations}</strong> {summary.generations === 1 ? 'generation' : 'generations'}
      </span>
      <span className={styles.stat}>
        {familyLines.first} &amp; {familyLines.second} lines
      </span>
      {earliest && (
        <span className={styles.stat}>
          Earliest record: {earliest.circa ? 'c. ' : ''}
          {earliest.year}
          {earliest.place ? `, ${earliest.place}` : ''}
        </span>
      )}
    </div>
  )
}

function Card({ person, relation, you }: { person: TreePerson; relation: string | null; you: boolean }) {
  const side = person.side ? sides[person.side] : null
  return (
    <Link to={`/people/${person.id}`} className={you ? `${styles.card} ${styles.cardYou}` : styles.card}>
      <span aria-hidden="true" className={`${styles.band} ${you ? styles.bandYou : (side?.band ?? '')}`}>
        <svg viewBox="0 0 200 64" preserveAspectRatio="none" className={styles.mountain}>
          <path d="M0 58 Q26 30 54 42 Q88 12 120 36 Q152 20 200 48" vectorEffect="non-scaling-stroke" />
        </svg>
      </span>
      <span aria-hidden="true" className={you ? `${styles.portrait} ${styles.portraitYou}` : styles.portrait}>
        {initials(person)}
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
