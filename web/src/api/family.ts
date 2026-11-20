import { api } from './client'

export type FamilySummary = {
  people: number
  generations: number
  earliest_record: { year: number; circa: boolean; place: string | null } | null
}

export const familyKeys = {
  summary: ['family', 'summary'] as const,
}

export const getFamilySummary = ({ signal }: { signal?: AbortSignal } = {}) =>
  api<FamilySummary>('GET', '/family/summary', { signal })
