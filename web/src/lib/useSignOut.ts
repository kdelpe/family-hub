import { useIsMutating, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router'

import { endSession, logout } from '../api/auth'
import { ApiError } from '../api/client'

// Shared by sign-out and a password change: one at a time, since a change answering after sign-out would set a
// fresh cookie and sign them back in.
export const signInKey = ['sign-in'] as const

export const useSignInBusy = () => useIsMutating({ mutationKey: signInKey }) > 0

export function useSignOut() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  return useMutation({
    mutationKey: signInKey,
    mutationFn: logout,
    // Off this page first, so the shell isn't left to refetch with the ended session.
    onSuccess: async () => {
      await navigate('/auth/login', { replace: true })
      endSession(queryClient)
    },
    // The server clears the cookie before ending the session, so its own failure still signed them out. A
    // proxy's or the cross-origin check's refusal never reached it.
    onError: async (error) => {
      if (error instanceof ApiError && error.status === 500 && error.code === 'internal') {
        await navigate('/auth/login', { replace: true })
        endSession(queryClient)
      }
    },
  })
}
