import { describe, expect, it } from 'vitest'

import { fullName, initials, nameMatches } from './names'

describe('names', () => {
  it.each([
    { name: 'Nadège Pluviose', query: 'nadege', want: true },
    { name: 'Stanley Jean-Louis', query: 'jean louis', want: true },
    { name: 'Stanley Jean-Louis', query: 'louis stan', want: true },
    { name: 'Naïka Jean-Louis', query: 'NAI', want: true },
    { name: 'Maya Pluviose', query: 'aya', want: false },
    { name: 'Maya Pluviose', query: '', want: true },
  ])('"$query" matches $name: $want', ({ name, query, want }) => {
    expect(nameMatches(name, query)).toBe(want)
  })

  it('writes full names and initials', () => {
    expect(fullName({ first_name: 'Rose', last_name: 'Monestime' })).toBe('Rose Monestime')
    expect(fullName({ first_name: 'Rose', last_name: null })).toBe('Rose')
    expect(initials({ first_name: 'naïka', last_name: 'Jean-Louis' })).toBe('NJ')
  })
})
