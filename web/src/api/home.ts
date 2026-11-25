import { api } from './client'

// A coming birthday: its date this year (or next) and how far off, never the birth year.
export type Birthday = { id: string; first_name: string; last_name: string | null; on: string; in_days: number }

export type Newcomer = { id: string; first_name: string; last_name: string | null; added_at: string }

export type Counts = { people: number; generations: number; new_this_month: number }

export type Home = { birthdays: Birthday[]; newcomers: Newcomer[]; counts: Counts }

export const homeKeys = {
  all: ['home'] as const,
}

export const homeQuery = {
  queryKey: homeKeys.all,
  queryFn: ({ signal }: { signal?: AbortSignal }) => api<Home>('GET', '/home', { signal }),
}
