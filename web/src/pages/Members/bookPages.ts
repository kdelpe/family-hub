import type { DirectoryEntry } from '../../api/contacts'
import type { TreePerson } from '../../api/tree'
import { foldName, fullName } from '../../lib/names'

export type Living = { entry: DirectoryEntry; relation: string | null }
export type Remembered = { person: TreePerson; relation: string | null }

export type BookPage =
  | {
      kind: 'empty' | 'cover' | 'endpaperFront' | 'welcome' | 'contents' | 'divider' | 'notes' | 'endpaperBack' | 'back'
    }
  | { kind: 'person'; living: Living; letter: string }
  | { kind: 'memorial'; remembered: Remembered }

export type Book = {
  pages: BookPage[]
  living: Living[]
  remembered: Remembered[]
  // Indexes into pages.
  welcome: number
  membersStart: number
  memoryStart: number
}

export const letterOf = (firstName: string) => foldName(firstName).charAt(0).toUpperCase() || '#'

const byName = (a: { first_name: string; last_name: string | null }, b: { first_name: string; last_name: string | null }) =>
  fullName(a).localeCompare(fullName(b), undefined, { sensitivity: 'base' })

// The book page by page (design-notes §4.3), laid out as spreads: index 0 is the empty space left of the closed
// cover, and a Notes page evens out the pages so the back cover lands on a right-hand page.
export function makeBook(living: Living[], remembered: Remembered[]): Book {
  const people = [...living].sort((a, b) => byName(a.entry, b.entry))
  const gone = [...remembered].sort((a, b) => byName(a.person, b.person))
  const pages: BookPage[] = [{ kind: 'empty' }, { kind: 'cover' }, { kind: 'endpaperFront' }, { kind: 'welcome' }, { kind: 'contents' }]
  const membersStart = pages.length
  people.forEach((l) => pages.push({ kind: 'person', living: l, letter: letterOf(l.entry.first_name) }))
  const memoryStart = pages.length
  pages.push({ kind: 'divider' })
  gone.forEach((r) => pages.push({ kind: 'memorial', remembered: r }))
  while (pages.length % 2 === 0) {
    pages.push({ kind: 'notes' })
  }
  pages.push({ kind: 'endpaperBack' }, { kind: 'back' }, { kind: 'empty' })
  return { pages, living: people, remembered: gone, welcome: 3, membersStart, memoryStart }
}

const numbered = new Set<BookPage['kind']>(['welcome', 'contents', 'person', 'divider', 'memorial', 'notes'])

// "1" for the Welcome page; covers and endpapers have none.
export const pageNumber = (book: Book, index: number) =>
  numbered.has(book.pages[index]!.kind) ? String(index - book.welcome + 1) : null

// Where the book can rest: a spread of two pages, or on a phone one page at a time (no blank or Notes pages).
export const stopsOf = (book: Book, single: boolean): number[][] =>
  single
    ? book.pages.flatMap((p, i) => (p.kind === 'empty' || p.kind === 'notes' ? [] : [[i]]))
    : Array.from({ length: book.pages.length / 2 }, (_, s) => [2 * s, 2 * s + 1])

// The stop showing a page, or the next one when it isn't shown on its own (the blank before the cover on a phone).
export const stopOf = (stops: number[][], index: number) =>
  Math.max(
    0,
    stops.findIndex((s) => s.at(-1)! >= index),
  )

export const nameOf = (page: BookPage) =>
  page.kind === 'person' ? page.living.entry : page.kind === 'memorial' ? page.remembered.person : null
