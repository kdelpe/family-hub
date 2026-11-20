import { QueryClient } from '@tanstack/react-query'
import { waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys, endSession, followSessionChanges, sameSession, sessionGeneration, startSession, type User } from './auth'

const before: User = { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: 'maya' }
const after: User = { id: 'u2', email: 'fritz.pluviose@example.com', role: 'member', person_id: 'fritz' }

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

it.each([
  { name: 'a person as the last viewer saw them', key: ['persons', 'nadege'], kept: false },
  { name: 'contacts shared with the last viewer', key: ['contacts', 'nadege'], kept: false },
  { name: 'the tree', key: ['tree'], kept: false },
  { name: 'the link being used to sign in', key: authKeys.preview('link-token'), kept: true },
])('a new session keeps $name: $kept', ({ key, kept }) => {
  const client = new QueryClient()
  client.setQueryData(authKeys.me, before)
  client.setQueryData(key, { cached: true })

  startSession(client, after)

  expect(client.getQueryData(key)).toEqual(kept ? { cached: true } : undefined)
  expect(client.getQueryData(authKeys.me)).toEqual(after)
})

it('makes another tab drop what it fetched for the last person, even an answer still on its way', async () => {
  const otherTab = new QueryClient()
  const stop = followSessionChanges(otherTab)
  otherTab.setQueryData(authKeys.me, before)
  otherTab.setQueryData(['contacts', 'nadege'], [{ value: '+1 718 555 0123' }])
  otherTab.setQueryData(authKeys.preview('link-token'), { purpose: 'invite' })
  let answer: (value: unknown) => void = () => {}
  void otherTab.prefetchQuery({ queryKey: ['persons', 'nadege'], queryFn: () => new Promise((resolve) => (answer = resolve)) })

  startSession(new QueryClient(), after)
  // A browser passes the signing-in tab's storage write on to every other tab as this event.
  window.dispatchEvent(new StorageEvent('storage', { key: 'fh-session-changed', newValue: localStorage.getItem('fh-session-changed') }))

  await waitFor(() => expect(otherTab.getQueryData(['contacts', 'nadege'])).toBeUndefined())
  answer({ id: 'nadege', birth_date: '1950-03-14' })
  await new Promise((resolve) => setTimeout(resolve, 0))
  expect(otherTab.getQueryData(['persons', 'nadege'])).toBeUndefined()
  expect(otherTab.getQueryData(authKeys.me)).toBeUndefined()
  // A link preview isn't anyone's data, and that tab may be halfway through setting a password with it.
  expect(otherTab.getQueryData(authKeys.preview('link-token'))).toEqual({ purpose: 'invite' })
  stop()
})

it('signs in even when the browser blocks storage', () => {
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new DOMException('blocked', 'SecurityError')
  })
  const client = new QueryClient()

  startSession(client, after)

  expect(client.getQueryData(authKeys.me)).toEqual(after)
})

describe('sameSession', () => {
  it('turns down an answer that started before the session changed', () => {
    const queryClient = new QueryClient()
    const since = sessionGeneration()
    expect(sameSession(since)).toBe(true)

    endSession(queryClient)

    expect(sameSession(since)).toBe(false)
    expect(sameSession(sessionGeneration())).toBe(true)
  })
})
