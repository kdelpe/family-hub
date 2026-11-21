import { api } from './client'

export type FamilySummary = {
  people: number
  generations: number
  earliest_record: { year: number; circa: boolean; place: string | null } | null
}

export const familyKeys = {
  summary: ['family', 'summary'] as const,
}

export const familySummaryQuery = {
  queryKey: familyKeys.summary,
  queryFn: ({ signal }: { signal?: AbortSignal }) => getFamilySummary({ signal }),
}

export const getFamilySummary = ({ signal }: { signal?: AbortSignal } = {}) =>
  api<FamilySummary>('GET', '/family/summary', { signal })
