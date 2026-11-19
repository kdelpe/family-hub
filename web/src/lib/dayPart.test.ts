import { describe, expect, it } from 'vitest'

import { dayPartLabel, greeting } from './dayPart'

describe('dayPart', () => {
  it.each([
    { time: '2026-11-20T04:59', label: 'Friday night', greeting: 'Good evening' },
    { time: '2026-11-20T05:00', label: 'Friday morning', greeting: 'Good morning' },
    { time: '2026-11-20T11:59', label: 'Friday morning', greeting: 'Good morning' },
    { time: '2026-11-20T12:00', label: 'Friday afternoon', greeting: 'Good afternoon' },
    { time: '2026-11-20T17:00', label: 'Friday evening', greeting: 'Good evening' },
    { time: '2026-11-20T22:00', label: 'Friday night', greeting: 'Good evening' },
  ])('$time is "$label" and "$greeting"', ({ time, label, greeting: want }) => {
    const date = new Date(time)

    expect(dayPartLabel(date)).toBe(label)
    expect(greeting(date)).toBe(want)
  })
})
