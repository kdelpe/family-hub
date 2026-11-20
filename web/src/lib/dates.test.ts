import { describe, expect, it } from 'vitest'

import type { Person } from '../api/persons'
import { birthday, bornOn, lifespan } from './dates'

describe('birthday', () => {
  it.each([
    { masked: '1960-11-19', style: 'long' as const, want: 'November 19' },
    { masked: '--02-02', style: 'short' as const, want: 'Feb 2' },
    { masked: '--11', style: 'long' as const, want: 'November' },
    { masked: '1960-11', style: 'short' as const, want: 'Nov' },
    { masked: '1960', style: 'long' as const, want: null },
    { masked: null, style: 'long' as const, want: null },
  ])('$masked ($style): $want', ({ masked, style, want }) => {
    expect(birthday(masked, style)).toBe(want)
  })
})

describe('lifespan', () => {
  const person = (over: Partial<Person>): Person => ({
    id: 'p',
    first_name: 'Lucien',
    last_name: 'Monestime',
    maiden_name: null,
    birth_place: null,
    birth_date: null,
    birth_date_precision: null,
    death_date: null,
    death_date_precision: null,
    is_deceased: true,
    bio: null,
    ...over,
  })

  it.each([
    { name: 'circa birth', over: { birth_date: '1935', birth_date_precision: 'circa' as const, death_date: '1998' }, want: 'c. 1935 – 1998' },
    { name: 'exact dates, years only', over: { birth_date: '1935-03-02', birth_date_precision: 'day' as const, death_date: '1998-07-01' }, want: '1935 – 1998' },
    { name: 'unknown birth', over: { death_date: '1998' }, want: '? – 1998' },
    { name: 'nothing known', over: {}, want: null },
  ])('$name: $want', ({ over, want }) => {
    expect(lifespan(person(over))).toBe(want)
  })
})

describe('bornOn', () => {
  it.each([
    { date: '1968-06-06', precision: 'day' as const, want: 'June 6, 1968' },
    { date: '1968-06-01', precision: 'month' as const, want: 'June 1968' },
    { date: '1968-01-01', precision: 'year' as const, want: '1968' },
    { date: '1968-01-01', precision: 'circa' as const, want: 'c. 1968' },
    { date: '--06-06', precision: 'day' as const, want: 'June 6' },
    { date: null, precision: null, want: null },
  ])('$date ($precision): $want', ({ date, precision, want }) => {
    expect(bornOn(date, precision)).toBe(want)
  })
})
