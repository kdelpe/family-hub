import { api } from './client'

// The fields the pages use so far; the API sends more (architecture §4).
export type Person = {
  id: string
  first_name: string
  last_name: string | null
}

export const personKeys = {
  one: (id: string) => ['persons', id] as const,
}

export const getPerson = (id: string, { signal }: { signal?: AbortSignal } = {}) =>
  api<Person>('GET', `/persons/${encodeURIComponent(id)}`, { signal })
