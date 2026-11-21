import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { directoryQuery } from '../../api/contacts'
import { treeQuery } from '../../api/tree'
import { errorMessage } from '../../lib/errorMessage'
import { useMotionPaused, useReducedMotion } from '../../lib/motion'
import { relationsTo } from '../../lib/relation'
import { useNarrow } from '../../lib/useNarrow'
import { useSignedIn } from '../../lib/useSignedIn'
import { Book } from './Book'
import { makeBook } from './bookPages'
import styles from './Members.module.css'

// Everyone in one place (design-notes §4.3): the living A to Z with how to reach them, and those remembered, as a
// book that fits the window. It replaces the People grid and the Directory.
export function Members() {
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const directory = useQuery(directoryQuery)
  const tree = useQuery(treeQuery)
  const [paused] = useMotionPaused()
  const reduced = useReducedMotion()
  const single = useNarrow()
  // Kept by page key, not index, so a refetch adding or removing someone leaves the book open at the same person.
  const [place, setPlace] = useState({ key: 'empty-0', index: 0 })

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

  const error = directory.error ?? tree.error

  return (
    <main className={styles.main}>
      <title>Members — Pluviose-Monestime Family Hub</title>
      <h1 className="visually-hidden">Members</h1>
      {error && <p role="alert">{errorMessage(error)}</p>}
      {book && <Book book={book} single={single} at={at} onAt={setAt} viewerId={viewerId} paused={paused} reduced={reduced} />}
    </main>
  )
}
