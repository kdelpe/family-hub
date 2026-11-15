import { expect, it } from 'vitest'

import { passwordStrength } from './passwordStrength'

it.each([
  { name: 'empty', password: '', want: { level: 0, label: '' } },
  { name: 'under the minimum', password: 'kafe123', want: { level: 1, label: 'Too short' } },
  { name: 'the minimum, one kind', password: 'kafekafe', want: { level: 2, label: 'Fair' } },
  { name: 'ten, three kinds', password: 'Kafe-kafe1', want: { level: 3, label: 'Good' } },
  { name: 'twelve, one kind', password: 'kafeakpenkaf', want: { level: 3, label: 'Good' } },
  { name: 'twelve, three kinds', password: 'Kafe-ak-pen1', want: { level: 4, label: 'Strong' } },
  { name: 'over the maximum', password: 'k'.repeat(129), want: { level: 1, label: 'Too long' } },
  { name: 'a long phrase', password: 'mango tree on rue sainte-anne', want: { level: 4, label: 'Strong' } },
  { name: 'counts characters, not bytes', password: 'kafé', want: { level: 1, label: 'Too short' } },
  { name: 'accented letters are letters, not symbols', password: 'kafèkafè12', want: { level: 2, label: 'Fair' } },
  { name: 'a decomposed accent is still part of its letter', password: 'kafe\u0301kafe\u030112', want: { level: 2, label: 'Fair' } },
  { name: 'a mark NFC can’t compose is not a symbol', password: 'kaf\u0331kaf\u033112', want: { level: 2, label: 'Fair' } },
  { name: 'counts after NFC, as the server does', password: 'kafekae\u0301', want: { level: 1, label: 'Too short' } },
])('$name', ({ password, want }) => {
  expect(passwordStrength(password)).toEqual(want)
})
