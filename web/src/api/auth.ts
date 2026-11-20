import type { Query, QueryClient } from '@tanstack/react-query'

import { api, ApiError } from './client'

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

const previewKey = ['auth', 'preview'] as const

export const authKeys = {
  me: ['auth', 'me'] as const,
  preview: (token: string) => [...previewKey, token] as const,
}

const sessionChangedKey = 'fh-session-changed'

// A link preview depends on its token, not on who's signed in, and a tab may be mid-way through setting a password
// with it; everything else was fetched as someone.
const fetchedAsSomeone = (query: Query) => query.queryKey[0] !== previewKey[0] || query.queryKey[1] !== previewKey[1]

// Everything fetched as whoever was signed in before (birth years, shared phone numbers) goes, so the next person on a
// shared computer never sees it, not even briefly.
export function startSession(queryClient: QueryClient, user: User) {
  dropSessionData(queryClient)
  queryClient.setQueryData(authKeys.me, user)
}

// Signed out here, so this tab and every other one drops what it fetched as this person.
export const endSession = (queryClient: QueryClient) => dropSessionData(queryClient)

function dropSessionData(queryClient: QueryClient) {
  queryClient.removeQueries({ predicate: fetchedAsSomeone })
  try {
    // The cookie changed for every tab; writing this fires a storage event in the others (followSessionChanges).
    localStorage.setItem(sessionChangedKey, String(Date.now()))
  } catch {
    // Storage can be blocked; the other tabs then catch up on their next fetch.
  }
}

// A save answered 401: the session ended elsewhere. Everything fetched as them goes, as at sign-out, and the
// shell, rechecking who's signed in, sends them to sign in.
export const onSessionEnded = (queryClient: QueryClient) => (error: Error) => {
  if (error instanceof ApiError && error.status === 401) {
    // Kept, then refetched: the shell is watching it, and a removed query wouldn't be fetched again.
    const isMe = (query: Query) => query.queryKey[0] === authKeys.me[0] && query.queryKey[1] === authKeys.me[1]
    queryClient.removeQueries({ predicate: (query) => fetchedAsSomeone(query) && !isMe(query) })
    void queryClient.invalidateQueries({ queryKey: authKeys.me })
  }
}

// Another tab signed someone in or out, so this tab's cookie changed: drop everything it fetched for the last person
// (a reset also stops any answer still on its way) and fetch again, as the new one or into a 401 and sign-in.
export function followSessionChanges(queryClient: QueryClient): () => void {
  const onStorage = (event: StorageEvent) => {
    if (event.key === sessionChangedKey) {
      void queryClient.resetQueries({ predicate: fetchedAsSomeone })
    }
  }
  window.addEventListener('storage', onStorage)
  return () => window.removeEventListener('storage', onStorage)
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

export const logout = () => api<void>('POST', '/auth/logout')

// Signs out every other session too; this one gets a new cookie.
export const changePassword = (currentPassword: string, newPassword: string) =>
  api<void>('POST', '/auth/password/change', { body: { current_password: currentPassword, new_password: newPassword } })
