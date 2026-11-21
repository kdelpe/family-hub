import { api, ApiError } from './client'
import type { Person } from './persons'

export type Relation = 'parent' | 'child' | 'partner' | 'sibling'
export type ParentKind = 'biological' | 'adoptive' | 'foster'

export type NewRelative = {
  relation: Relation
  person_id?: string
  person?: { first_name: string; last_name: string | null }
  subtype?: ParentKind | 'married' | 'partner'
  ended?: boolean
  other_parent_id?: string
  shared_parent_id?: string
}

export type AddedRelative = { person: Person; relationships: { id: string }[] }

export const addRelative = (personId: string, relative: NewRelative) =>
  api<AddedRelative>('POST', `/persons/${encodeURIComponent(personId)}/relatives`, { body: relative })

// Undo: the links go, and a person added just now goes too (a soft delete, so it can be restored).
export async function undoRelative(added: AddedRelative, isNew: boolean) {
  const paths = [
    ...added.relationships.map((link) => `/relationships/${encodeURIComponent(link.id)}`),
    ...(isNew ? [`/persons/${encodeURIComponent(added.person.id)}`] : []),
  ]
  for (const path of paths) {
    await api<void>('DELETE', path).catch((error: unknown) => {
      // Gone already: an earlier Undo got this far before failing, so a retry carries on from here.
      if (!(error instanceof ApiError && error.status === 404)) {
        throw error
      }
    })
  }
}
