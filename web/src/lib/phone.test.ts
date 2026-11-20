import { describe, expect, it } from 'vitest'

import { formatPhone } from './phone'

describe('formatPhone', () => {
  it.each([
    { typed: '1111111111', want: '111-111-1111' },
    { typed: '15550107781', want: '1-555-010-7781' },
    { typed: '(555) 010-7781', want: '555-010-7781' },
    { typed: '555 010 7781', want: '555-010-7781' },
    { typed: '111555', want: '111555' },
    { typed: '3412-3456', want: '3412-3456' },
    { typed: '25550107781', want: '25550107781' },
    { typed: '555-010-77812', want: '55501077812' },
    { typed: '1-111-555-000', want: '1-111-555-000' },
    { typed: '1-115-550-00012', want: '111555000012' },
    { typed: '+509 3412 3456', want: '+509 3412 3456' },
    { typed: '(', want: '(' },
    { typed: '', want: '' },
    { typed: '555 abc', want: '555 abc' },
  ])('$typed → $want', ({ typed, want }) => {
    expect(formatPhone(typed)).toBe(want)
  })
})
