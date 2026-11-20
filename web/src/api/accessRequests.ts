import { api } from './client'

export type FamilySide = 'first' | 'second' | 'married_in'

export type AccessRequest = {
  full_name: string
  email: string
  side: FamilySide | null
  relationship: string
  related_names: string[]
  note: string
  // A honeypot: hidden from people, so only a bot fills it in, and the server then drops the request.
  website: string
}

// Always answers the same, whether or not the email has an account or a pending request.
export const requestAccess = (request: AccessRequest) => api<void>('POST', '/access-requests', { body: request })

// The server keeps to this list (internal/admin/access.go), so the admins' queue reads the same for every request.
export const relationships = [
  'daughter',
  'son',
  'grandchild',
  'sister',
  'brother',
  'niece or nephew',
  'cousin',
  'wife, husband or partner',
  'mother or father',
  'other relative',
]

export type Suggestion = Pick<AccessRequest, 'full_name' | 'email' | 'related_names' | 'note'> & {
  relationship: string | null
}

// Answers the same as the public form, however it ends: a repeat, or one past the day's limit, is dropped.
export const suggestRelative = (suggestion: Suggestion) => api<void>('POST', '/me/suggestions', { body: suggestion })
