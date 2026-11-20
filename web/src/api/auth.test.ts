import { QueryClient } from '@tanstack/react-query'
import { expect, it } from 'vitest'

import { authKeys, startSession, type User } from './auth'

const before: User = { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: 'maya' }
const after: User = { id: 'u2', email: 'fritz.pluviose@example.com', role: 'member', person_id: 'fritz' }

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
