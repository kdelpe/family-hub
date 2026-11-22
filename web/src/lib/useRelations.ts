import { useQuery } from '@tanstack/react-query'

import { type Tree, treeQuery } from '../api/tree'
import { relationsTo } from './relation'

// One map per tree read and viewer, shared by every name card open on the page.
const built = new WeakMap<Tree, Map<string, Map<string, string>>>()

function relationsFor(tree: Tree, viewerId: string) {
  let byViewer = built.get(tree)
  if (!byViewer) {
    byViewer = new Map()
    built.set(tree, byViewer)
  }
  let relations = byViewer.get(viewerId)
  if (!relations) {
    relations = relationsTo(tree, viewerId)
    byViewer.set(viewerId, relations)
  }
  return relations
}

// How each person is related to the viewer; fetches the whole tree only when enabled.
export function useRelations(viewerId: string | null, enabled = true) {
  // Each card opened mounts its own reader; a minute's freshness spares a whole-tree read per name, and a
  // relationship change still invalidates it.
  const tree = useQuery({ ...treeQuery, staleTime: 60_000, enabled: enabled && viewerId !== null })
  const relations = tree.data && viewerId ? relationsFor(tree.data, viewerId) : null
  return { relations, isPending: tree.isPending }
}
