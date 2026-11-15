import { queryOptions } from '@tanstack/react-query'

import { api } from './client'

export type User = {
  id: string
  email: string
  role: 'admin' | 'member'
}

export type Providers = {
  google: boolean
}

export type LinkPreview = {
  purpose: 'invite' | 'password_reset'
  invitee_email: string
  invitee_first_name: string | null
  from_name: string | null
  google_eligible: boolean
}

export const authKeys = {
  providers: ['auth', 'providers'] as const,
  preview: (token: string) => ['auth', 'preview', token] as const,
}

// A full-page navigation, not a fetch: the server answers with a redirect to Google.
export const googleStartPath = '/api/auth/google/start'

export const login = (email: string, password: string) =>
  api<User>('POST', '/auth/login', { body: { email, password } })

// Always answers the same, whether or not the email has an account.
export const forgotPassword = (email: string) =>
  api<void>('POST', '/auth/password/forgot', { body: { email } })

export const providers = ({ signal }: { signal?: AbortSignal } = {}) =>
  api<Providers>('GET', '/auth/providers', { signal })

export const providersQuery = queryOptions({
  queryKey: authKeys.providers,
  queryFn: providers,
  staleTime: Infinity,
  // One quick check: retrying would hold back the Google button and its messages for seconds.
  retry: false,
  // Not paused while the browser says it's offline, which would keep the Google parts hidden.
  networkMode: 'always',
})

// Reads an invite or reset link without using it up.
export const previewLink = (token: string, { signal }: { signal?: AbortSignal } = {}) =>
  api<LinkPreview>('POST', '/auth/invite/preview', { body: { token }, signal })

export const setPassword = (token: string, password: string) =>
  api<User>('POST', '/auth/password/reset', { body: { token, password } })
