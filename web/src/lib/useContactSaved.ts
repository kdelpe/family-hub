import { useQueryClient } from '@tanstack/react-query'

import { onSessionEnded } from '../api/auth'
import { contactKeys, type ContactMethod, directoryKeys } from '../api/contacts'

// After saving one of your own contact rows: the cached list updated, and every read of it refreshed.
export function useContactSaved(personId: string) {
  const queryClient = useQueryClient()
  return {
    saved: (update: (rows: ContactMethod[]) => ContactMethod[]) => {
      queryClient.setQueryData<ContactMethod[]>(contactKeys.mine(personId), (rows) => rows && update(rows))
      // Your own rows too: a refresh that read them before this save mustn't land after it.
      for (const queryKey of [contactKeys.mine(personId), contactKeys.of(personId), directoryKeys.all]) {
        void queryClient.invalidateQueries({ queryKey })
      }
    },
    onError: onSessionEnded(queryClient),
  }
}
