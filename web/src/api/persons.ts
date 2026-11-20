import { api } from './client'

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
  all: ['persons'] as const,
  one: (id: string) => ['persons', id] as const,
}

export const getPerson = (id: string, { signal }: { signal?: AbortSignal } = {}) =>
  api<Person>('GET', `/persons/${encodeURIComponent(id)}`, { signal })

// Everyone, A–Z, with masked dates: for the notes under each relative's name.
export const listPersons = async ({ signal }: { signal?: AbortSignal } = {}) =>
  (await api<{ persons: Person[] }>('GET', '/persons', { signal })).persons
