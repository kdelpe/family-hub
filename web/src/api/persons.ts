import { api } from './client'

// The fields the pages use so far; the API also sends middle_name, side and gender (architecture §4).
// A birth date masked by the server (§5.2): "1960-11-19", "1960-11", "1960", or "--11-19" / "--11" without the year.
export type Person = {
  id: string
  first_name: string
  last_name: string | null
  maiden_name: string | null
  birth_place: string | null
  birth_date: string | null
  birth_date_precision: 'day' | 'month' | 'year' | 'circa' | null
  death_date: string | null
  death_date_precision: 'day' | 'month' | 'year' | 'circa' | null
  is_deceased: boolean
  bio: string | null
}

export const personKeys = {
  one: (id: string) => ['persons', id] as const,
  some: (ids: string[]) => ['persons', 'ids', ...ids] as const,
}

export const getPerson = (id: string, { signal }: { signal?: AbortSignal } = {}) =>
  api<Person>('GET', `/persons/${encodeURIComponent(id)}`, { signal })

// The server answers at most this many ids at a time.
const idsPerRequest = 100

// Just these people, with masked dates: the family is too big to fetch everyone for a few names.
export const getPersons = async (ids: string[], { signal }: { signal?: AbortSignal } = {}) => {
  const batches: string[][] = []
  for (let i = 0; i < ids.length; i += idsPerRequest) {
    batches.push(ids.slice(i, i + idsPerRequest))
  }
  const answers = await Promise.all(
    batches.map((batch) =>
      api<{ persons: Person[] }>('GET', `/persons?ids=${batch.map(encodeURIComponent).join(',')}`, { signal }),
    ),
  )
  return answers.flatMap((answer) => answer.persons)
}
