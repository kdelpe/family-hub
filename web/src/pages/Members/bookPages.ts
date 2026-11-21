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
  // Each letter's first page, A to Z.
  letterStarts: { letter: string; index: number }[]
  // A name for each page that survives a refetch adding or removing people: a person's id, or the page's kind.
  keys: string[]
  indexOf: Map<string, number>
}

export const letterOf = (firstName: string) => foldName(firstName).charAt(0).toUpperCase() || '#'

// By first name, as the letters go ("Anna" before "Anna Maria"), then by last name.
const byName = (a: { first_name: string; last_name: string | null }, b: { first_name: string; last_name: string | null }) =>
  a.first_name.localeCompare(b.first_name, undefined, { sensitivity: 'base' }) ||
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
  const letterStarts: Book['letterStarts'] = []
  pages.forEach((p, index) => {
    if (p.kind === 'person' && p.letter !== letterStarts.at(-1)?.letter) {
      letterStarts.push({ letter: p.letter, index })
    }
  })
  const keys = pages.map((p, i) => nameOf(p)?.id ?? (p.kind === 'empty' || p.kind === 'notes' ? `${p.kind}-${i === 0 ? 0 : pages.length - i}` : p.kind))
  return {
    pages,
    living: people,
    remembered: gone,
    welcome: 3,
    membersStart,
    memoryStart,
    letterStarts,
    keys,
    indexOf: new Map(keys.map((k, i) => [k, i])),
  }
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

// --book-page-1 to -8 in tokens.css: pages take them in turn from Welcome on (its cream sits well beside the
// endpaper's green), and so do the letter tabs.
export const pageTints = 8
