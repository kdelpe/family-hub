import { api } from './client'

export type User = {
  id: string
  email: string
  role: 'admin' | 'member'
}

export type Providers = {
  google: boolean
}

export const authKeys = {
  providers: ['auth', 'providers'] as const,
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
