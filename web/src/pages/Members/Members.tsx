import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { directoryQuery } from '../../api/contacts'
import { treeQuery } from '../../api/tree'
import { errorMessage } from '../../lib/errorMessage'
import { useMotionPaused, useReducedMotion } from '../../lib/motion'
import { fullName, initials, nameMatches } from '../../lib/names'
import { relationsTo } from '../../lib/relation'
import { useNarrow } from '../../lib/useNarrow'
import { useSignedIn } from '../../lib/useSignedIn'
import { Book } from './Book'
import { makeBook, nameOf } from './bookPages'
import styles from './Members.module.css'

// Everyone in one place (design-notes §4.3): the living A to Z with how to reach them, and those remembered, as a
// book or a list. It replaces the People grid and the Directory.
export function Members() {
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const directory = useQuery(directoryQuery)
  const tree = useQuery(treeQuery)
  const [paused] = useMotionPaused()
  const reduced = useReducedMotion()
  const single = useNarrow()
  const [view, setView] = useState<'book' | 'list'>('book')
  // Kept by page key, not index, so a refetch adding or removing someone leaves the book open at the same person.
  const [place, setPlace] = useState({ key: 'empty-0', index: 0 })
  const [query, setQuery] = useState('')

  const book = useMemo(() => {
    // Only after a fetch that worked: a failed refresh mustn't keep showing what was hidden or taken back since.
    if (!directory.isSuccess || !tree.data) {
      return null
    }
    const relations = viewerId ? relationsTo(tree.data, viewerId) : new Map<string, string>()
    const living = directory.data.map((entry) => ({ entry, relation: relations.get(entry.id) ?? null }))
    const remembered = tree.data.persons
      .filter((p) => p.is_deceased)
      .map((person) => ({ person, relation: relations.get(person.id) ?? null }))
    return makeBook(living, remembered)
  }, [directory.isSuccess, directory.data, tree.data, viewerId])

  const at = book ? (book.indexOf.get(place.key) ?? Math.min(place.index, book.pages.length - 1)) : 0
  const setAt = (index: number) => book && setPlace({ key: book.keys[index]!, index })
  const openAt = (index: number) => {
    setView('book')
    setAt(index)
  }
  const pageOf = (id: string) => book?.indexOf.get(id) ?? 0

  const error = directory.error ?? tree.error

  return (
    <main className={styles.main}>
      <title>Members — Pluviose-Monestime Family Hub</title>
      <div className={styles.top}>
        <div className={styles.heading}>
          <h1 className={styles.title}>Members</h1>
          <span className={styles.subtitle}>Everyone in the family, A to Z, and those we remember.</span>
        </div>
        <label htmlFor="mb-search" className="visually-hidden">
          Find someone
        </label>
        <input
          id="mb-search"
          type="search"
          placeholder="Find someone…"
          autoComplete="off"
          value={query}
          onChange={(event) => {
            const q = event.target.value
            setQuery(q)
            // Opens the book at the first match, as it's typed.
            const hit = q.trim() && book ? book.pages.findIndex((p) => nameOf(p) && nameMatches(fullName(nameOf(p)!), q)) : -1
            if (hit >= 0) {
              openAt(hit)
            }
          }}
          className={styles.search}
        />
        <div role="group" aria-label="View" className={styles.views}>
          <button type="button" className={`pill ${styles.viewButton}`} aria-pressed={view === 'book'} onClick={() => setView('book')}>
            Book
          </button>
          <button type="button" className={`pill ${styles.viewButton}`} aria-pressed={view === 'list'} onClick={() => setView('list')}>
            List
          </button>
        </div>
      </div>

      {error && <p role="alert">{errorMessage(error)}</p>}
      {book &&
        (view === 'book' ? (
          <Book book={book} single={single} at={at} onAt={setAt} viewerId={viewerId} paused={paused} reduced={reduced} />
        ) : (
          <div className={styles.list}>
            <h2 className={styles.listLabel}>With us</h2>
            <ul className={styles.grid}>
              {book.living.map(({ entry, relation }) => (
                <li key={entry.id}>
                  <button type="button" className={styles.row} onClick={() => openAt(pageOf(entry.id))}>
                    <span aria-hidden="true" className={styles.rowAvatar}>
                      {initials(entry)}
                    </span>
                    <span className={styles.rowWords}>
                      <span className={styles.rowName}>{fullName(entry)}</span>
                      <span className={styles.rowSub}>{entry.id === viewerId ? 'You' : relation}</span>
                    </span>
                    <span className={styles.rowOpen}>Open ›</span>
                  </button>
                </li>
              ))}
            </ul>
            <h2 className={`${styles.listLabel} ${styles.listLabelLater}`}>In memory</h2>
            <ul className={styles.grid}>
              {book.remembered.map(({ person, relation }) => (
                <li key={person.id}>
                  <button type="button" className={styles.row} onClick={() => openAt(pageOf(person.id))}>
                    <span aria-hidden="true" className={`${styles.rowAvatar} ${styles.gray}`}>
                      {initials(person)}
                    </span>
                    <span className={styles.rowWords}>
                      <span className={styles.rowName}>{fullName(person)}</span>
                      <span className={styles.rowSub}>{relation ? `${relation} · In memory` : 'In memory'}</span>
                    </span>
                    <span className={styles.rowOpen}>Open ›</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ))}
    </main>
  )
}
