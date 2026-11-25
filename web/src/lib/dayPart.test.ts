import { describe, expect, it } from 'vitest'

import { greeting } from './dayPart'

describe('dayPart', () => {
  it.each([
    { time: '2026-11-20T04:59', greeting: 'Good evening' },
    { time: '2026-11-20T05:00', greeting: 'Good morning' },
    { time: '2026-11-20T11:59', greeting: 'Good morning' },
    { time: '2026-11-20T12:00', greeting: 'Good afternoon' },
    { time: '2026-11-20T17:00', greeting: 'Good evening' },
    { time: '2026-11-20T22:00', greeting: 'Good evening' },
  ])('$time greets "$greeting"', ({ time, greeting: want }) => {
    expect(greeting(new Date(time))).toBe(want)
  })
})
