import type { QueryClient } from '@tanstack/react-query'

import { authKeys } from './auth'
import { api, ApiError } from './client'
import type { TreePerson } from './tree'

export type RequestKind = 'phone' | 'whatsapp' | 'address'

export type ContactMethod = {
  id: string
  kind: 'email' | 'phone' | 'address' | 'social' | 'other'
  platform: string | null
  label: string | null
  value: string
  visibility: 'family' | 'on_request' | 'hidden'
  // The only link a page may make of a social account (§5.2), built by the server; null for anything else.
  href: string | null
  shared_with_you: boolean
}

// An on_request kind the viewer can't see yet, never its value (§5.2).
export type Requestable = { kind: RequestKind; requestable: true; my_request_status: 'none' | 'requested' }

export type Contact = ContactMethod | Requestable

export const isRequestable = (c: Contact): c is Requestable => 'requestable' in c

export const requestKinds: RequestKind[] = ['phone', 'whatsapp', 'address']

export const kindWords: Record<RequestKind, string> = { phone: 'Phone number', whatsapp: 'WhatsApp', address: 'Address' }

// Which kind of request a phone, WhatsApp or address row answers; null for anything else.
export const kindOf = (c: ContactMethod): RequestKind | null =>
  c.platform === 'whatsapp' ? 'whatsapp' : c.kind === 'phone' || c.kind === 'address' ? c.kind : null

export const contactKeys = {
  of: (personId: string) => ['contacts', personId] as const,
  // Whose rows too: an admin can relink a login, and /me/contacts then answers for the new person.
  mine: (personId: string) => ['contacts', 'mine', personId] as const,
}

export const directoryKeys = {
  all: ['directory'] as const,
}

export type DirectoryEntry = {
  id: string
  first_name: string
  last_name: string | null
  side: TreePerson['side']
  has_account: boolean
  contacts: Contact[]
}

// Never kept once the page is left, like a person's contacts.
export const directoryQuery = {
  queryKey: directoryKeys.all,
  queryFn: async ({ signal }: { signal?: AbortSignal }) =>
    (await api<{ people: DirectoryEntry[] }>('GET', '/directory', { signal })).people,
  gcTime: 0,
}

// Never kept once no page shows them (§5.2): each visit asks afresh, so a share taken back is gone at once.
export const contactsQuery = (personId: string) => ({
  queryKey: contactKeys.of(personId),
  queryFn: ({ signal }: { signal?: AbortSignal }) => getContacts(personId, { signal }),
  gcTime: 0,
})

export const getContacts = async (personId: string, { signal }: { signal?: AbortSignal } = {}) =>
  (await api<{ contacts: Contact[] }>('GET', `/persons/${encodeURIComponent(personId)}/contacts`, { signal })).contacts

export const askFor = (personId: string, kind: RequestKind) =>
  api<{ kind: RequestKind; my_request_status: 'requested' | 'shared' }>(
    'POST',
    `/persons/${encodeURIComponent(personId)}/contact-requests`,
    { body: { kind } },
  )

// Someone asking for one of my details: waiting for my answer, or answered.
export type ContactRequest = {
  id: string
  kind: RequestKind
  status: 'pending' | 'shared' | 'declined' | 'revoked'
  created_at: string
  decided_at: string | null
  from: { person_id: string; name: string }
}

export const contactRequestKeys = {
  // Whose, as for contactKeys.mine: a relinked login answers for the new person.
  mine: (personId: string) => ['contact-requests', personId] as const,
}

// Pending ones first, then the answered ones, each oldest first.
export const myRequestsQuery = (personId: string) => ({
  queryKey: contactRequestKeys.mine(personId),
  queryFn: async ({ signal, client }: MineContext) => {
    const answer = await readMine<{ person_id: string; requests: ContactRequest[] }>(client, personId, '/me/contact-requests', signal)
    return answer.requests
  },
})

export const decideRequest = (id: string, decision: 'share' | 'decline' | 'revoke') =>
  api<void>('POST', `/me/contact-requests/${encodeURIComponent(id)}/${decision}`)

const platformLabels: Record<string, string> = {
  instagram: 'Instagram',
  facebook: 'Facebook',
  tiktok: 'TikTok',
  linkedin: 'LinkedIn',
  x: 'X',
  youtube: 'YouTube',
  whatsapp: 'WhatsApp',
}

export const platformLabel = (platform: string | null) => (platform && platformLabels[platform]) || 'Other'

// A /me read that answered for someone other than the person shown: rows kept from before are no longer theirs,
// so pages hide them and offer nothing to act on.
export class NotYoursError extends Error {
  constructor(message = 'your profile didn’t load') {
    super(message)
  }
}

// Not a failure to show: the page is about to reload as whoever the login is linked to now.
export class RelinkedError extends NotYoursError {
  constructor() {
    super('your login is linked to someone else now')
  }
}

type MineContext = { signal?: AbortSignal; client: QueryClient }

// An admin can relink or unlink a login at any time (§5.1), so rows for someone else never go under this
// person's key: who you are is read again first, and the page moves on.
async function readMine<T extends { person_id: string }>(client: QueryClient, personId: string, path: string, signal?: AbortSignal) {
  const read = async () => {
    try {
      return await api<T>('GET', path, { signal })
    } catch (error) {
      // Unlinked: the server no longer has anyone to answer for.
      if (error instanceof ApiError && error.status === 403) {
        return null
      }
      throw error
    }
  }
  const answer = await read()
  if (answer?.person_id === personId) {
    return answer
  }
  await client.refetchQueries({ queryKey: authKeys.me })
  const now = client.getQueryData<{ person_id: string | null }>(authKeys.me)
  if (now && now.person_id !== personId) {
    throw new RelinkedError()
  }
  // Still them as far as we know (relinked and back, say): read once more before calling it a failure.
  const again = await read()
  if (again?.person_id === personId) {
    return again
  }
  throw new NotYoursError()
}

// Never kept once My profile is left, like everyone else's contacts (§5.2).
export const myContactsQuery = (personId: string) => ({
  queryKey: contactKeys.mine(personId),
  queryFn: async ({ signal, client }: MineContext) => {
    const answer = await readMine<{ person_id: string; contacts: ContactMethod[] }>(client, personId, '/me/contacts', signal)
    return answer.contacts
  },
  gcTime: 0,
})

export type NewContact = Pick<ContactMethod, 'kind' | 'platform' | 'label' | 'value' | 'visibility'>

export const addMyContact = (c: NewContact) => api<ContactMethod>('POST', '/me/contacts', { body: c })

export const updateMyContact = (id: string, patch: Partial<Pick<ContactMethod, 'label' | 'value' | 'visibility'>>) =>
  api<ContactMethod>('PATCH', `/me/contacts/${encodeURIComponent(id)}`, { body: patch })

export const removeMyContact = (id: string) => api<void>('DELETE', `/me/contacts/${encodeURIComponent(id)}`)
