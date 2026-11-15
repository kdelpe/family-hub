import { api } from './client'

export type FamilySide = 'first' | 'second' | 'married_in'

export type AccessRequest = {
  full_name: string
  email: string
  side: FamilySide | null
  relationship: string
  related_names: string[]
  note: string
}

// Always answers the same, whether or not the email has an account or a pending request.
export const requestAccess = (request: AccessRequest) => api<void>('POST', '/access-requests', { body: request })
