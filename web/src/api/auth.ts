import { api } from './client'

export type User = {
  id: string
  email: string
  role: 'admin' | 'member'
  // Who they are in the tree; null until an admin links them.
  person_id: string | null
}

export type LinkPreview = {
  purpose: 'invite' | 'password_reset'
  invitee_email: string
  invitee_first_name: string | null
  from_name: string | null
}

export const authKeys = {
  me: ['auth', 'me'] as const,
  preview: (token: string) => ['auth', 'preview', token] as const,
}

export const me = ({ signal }: { signal?: AbortSignal } = {}) => api<User>('GET', '/auth/me', { signal })

export const login = (email: string, password: string) =>
  api<User>('POST', '/auth/login', { body: { email, password } })

// Always answers the same, whether or not the email has an account.
export const forgotPassword = (email: string) =>
  api<void>('POST', '/auth/password/forgot', { body: { email } })

// Reads an invite or reset link without using it up.
export const previewLink = (token: string, { signal }: { signal?: AbortSignal } = {}) =>
  api<LinkPreview>('POST', '/auth/invite/preview', { body: { token }, signal })

export const setPassword = (token: string, password: string) =>
  api<User>('POST', '/auth/password/reset', { body: { token, password } })
