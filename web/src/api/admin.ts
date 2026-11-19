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
}

export const adminKeys = {
  members: ['admin', 'members'] as const,
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

export const invite = (input: InviteInput) => api<Member>('POST', '/admin/users', { body: input })

// A fresh invite for someone who never set a password, otherwise a password reset.
export const sendLink = (id: string) => api<void>('POST', `/admin/users/${encodeURIComponent(id)}/send-reset`)
