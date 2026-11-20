import { api } from './client'

export type RequestKind = 'phone' | 'whatsapp' | 'address'

export type ContactMethod = {
  id: string
  kind: 'email' | 'phone' | 'address' | 'social'
  platform: string | null
  label: string | null
  value: string
  visibility: 'family' | 'on_request' | 'hidden'
}

// An on_request kind the viewer can't see yet, never its value (§5.2).
export type Requestable = { kind: RequestKind; requestable: true; my_request_status: 'none' | 'requested' }

export type Contact = ContactMethod | Requestable

export const isRequestable = (c: Contact): c is Requestable => 'requestable' in c

export const contactKeys = {
  of: (personId: string) => ['contacts', personId] as const,
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
