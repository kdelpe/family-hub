import { api } from './client'

export type TreePerson = {
  id: string
  first_name: string
  last_name: string | null
  gender: 'male' | 'female' | 'other' | 'unknown' | null
  is_deceased: boolean
}

export type TreeRelationship =
  | { id: string; type: 'parent_child'; parent_id: string; child_id: string }
  | { id: string; type: 'couple'; ended: boolean; person_ids: [string, string] }

export type Tree = { persons: TreePerson[]; relationships: TreeRelationship[] }

export const treeKeys = {
  all: ['tree'] as const,
}

export const getTree = ({ signal }: { signal?: AbortSignal } = {}) => api<Tree>('GET', '/tree', { signal })
