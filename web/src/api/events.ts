import { api } from './client'

export type DatePrecision = 'day' | 'month' | 'year' | 'circa'

// Who was there: names only, A–Z.
export type EventPerson = { id: string; first_name: string; last_name: string | null }

// date is "YYYY-MM-DD", "YYYY-MM" or "YYYY", to match its precision.
export type FamilyEvent = {
  id: string
  title: string
  date: string | null
  date_precision: DatePrecision | null
  location: string | null
  description: string | null
  people: EventPerson[]
  created_at: string
}

export const eventKeys = {
  all: ['events'] as const,
}

// Newest first, undated last.
export const eventsQuery = {
  queryKey: eventKeys.all,
  queryFn: ({ signal }: { signal?: AbortSignal }) =>
    api<{ events: FamilyEvent[] }>('GET', '/events', { signal }).then((r) => r.events),
}

export type NewEvent = {
  title: string
  date?: string
  date_precision?: DatePrecision
  location?: string
  description?: string
  person_ids: string[]
}

export const createEvent = (body: NewEvent) => api<FamilyEvent>('POST', '/events', { body })
