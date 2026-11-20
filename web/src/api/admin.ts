import type { FamilySide } from './accessRequests'
import type { User } from './auth'
import { api } from './client'

export type Member = {
  id: string
  email: string
  role: 'admin' | 'member'
  person: { id: string; first_name: string; last_name: string | null } | null
  status: 'active' | 'invited' | 'disabled'
  // Only for invited: the latest link's expiry; past or null means none still works.
  invite_expires_at: string | null
}

export type InviteInput = {
  email: string
  role: 'admin' | 'member'
  person_id: string
  from_person_id?: string
  // The pending request this invite approves.
  access_request_id?: string
}

type PersonName = { person_id: string; name: string }

export type AccessRequestItem = {
  id: string
  full_name: string
  email: string
  side: FamilySide | null
  relationship: string | null
  // Each name they gave, with the closest person in the tree when one is close enough.
  related_names: { name: string; match: PersonName | null }[]
  note: string | null
  source: 'self' | 'member_suggestion'
  // Null on a suggestion once the member who made it has no login any more.
  suggested_by: PersonName | null
  status: 'pending' | 'declined'
  reviewed_at: string | null
  created_at: string
}

export const adminKeys = {
  members: ['admin', 'members'] as const,
  requests: ['admin', 'access-requests'] as const,
}

// Every page of the list: a family's members fit in a few.
export async function listMembers({ signal }: { signal?: AbortSignal } = {}): Promise<Member[]> {
  const members: Member[] = []
  let cursor: string | null = null
  do {
    const query: string = cursor === null ? '' : `&cursor=${encodeURIComponent(cursor)}`
    const page: { users: Member[]; next_cursor: string | null } = await api('GET', `/admin/users?limit=100${query}`, { signal })
    members.push(...page.users)
    cursor = page.next_cursor
  } while (cursor !== null)
  return members
}

// Answers with the new login as /auth/me shows one, not as the members list does.
export const invite = (input: InviteInput) => api<User>('POST', '/admin/users', { body: input })

// A fresh invite for someone who never set a password, otherwise a password reset.
export const sendLink = (id: string) => api<void>('POST', `/admin/users/${encodeURIComponent(id)}/send-reset`)

// Pending first, then those set aside in the last 90 days.
export const listAccessRequests = async ({ signal }: { signal?: AbortSignal } = {}) =>
  (await api<{ requests: AccessRequestItem[] }>('GET', '/admin/access-requests', { signal })).requests

export const declineRequest = (id: string) => api<void>('POST', `/admin/access-requests/${encodeURIComponent(id)}/decline`)

export const reopenRequest = (id: string) => api<void>('POST', `/admin/access-requests/${encodeURIComponent(id)}/reopen`)
