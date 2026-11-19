import { useQuery } from '@tanstack/react-query'

import { authKeys, me } from '../api/auth'
import { getPerson, personKeys } from '../api/persons'

// The signed-in user and who they are in the tree. The shell and its pages share the cached answers.
export function useSignedIn() {
  const user = useQuery({ queryKey: authKeys.me, queryFn: ({ signal }) => me({ signal }) })
  const personId = user.data?.person_id ?? null
  const person = useQuery({
    queryKey: personKeys.one(personId ?? ''),
    queryFn: ({ signal }) => getPerson(personId!, { signal }),
    enabled: personId !== null,
  })
  return {
    user: user.data,
    error: user.error,
    // Undefined while it loads, and for good if the login isn't linked or the read fails: pages fall back to less.
    person: person.data,
    personSettled: personId === null || !person.isPending,
  }
}
