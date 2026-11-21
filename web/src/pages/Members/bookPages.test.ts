import { describe, expect, it } from 'vitest'

import type { TreePerson } from '../../api/tree'
import { makeBook, nameOf } from './bookPages'

const remembered = (id: string, first_name: string, last_name: string) => ({
  person: { id, first_name, last_name, side: 'first', gender: 'female', is_deceased: true, bio_excerpt: null } as TreePerson,
  relation: null,
})

describe('makeBook', () => {
  it('orders people by first name before last name, so Anna comes before Anna Maria', () => {
    const book = makeBook([], [remembered('am', 'Anna Maria', 'Adams'), remembered('a', 'Anna', 'Zimmer'), remembered('b', 'Bea', 'Adams')])

    expect(book.pages.flatMap((p) => nameOf(p)?.first_name ?? [])).toEqual(['Anna', 'Anna Maria', 'Bea'])
  })
})
