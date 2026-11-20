import type { Tree } from '../api/tree'

export type Partner = { id: string; married: boolean; ended: boolean }
export type StepChild = { id: string; via: string }

export type FamilyGroups = {
  parents: string[]
  partners: Partner[]
  children: string[]
  stepChildren: StepChild[]
  grandchildren: string[]
}

// One person's close family for their profile (design-notes §4.9), over family-parent links (§3.4).
export function familyGroups(tree: Tree, personId: string): FamilyGroups {
  const parentsOf = new Map<string, string[]>()
  const childrenOf = new Map<string, string[]>()
  const partners: Partner[] = []
  for (const r of tree.relationships) {
    if (r.type === 'parent_child' && r.subtype !== 'foster') {
      parentsOf.set(r.child_id, [...(parentsOf.get(r.child_id) ?? []), r.parent_id])
      childrenOf.set(r.parent_id, [...(childrenOf.get(r.parent_id) ?? []), r.child_id])
    } else if (r.type === 'couple' && r.person_ids.includes(personId)) {
      const other = r.person_ids[0] === personId ? r.person_ids[1] : r.person_ids[0]
      partners.push({ id: other, married: r.subtype === 'married', ended: r.ended })
    }
  }
  const children = childrenOf.get(personId) ?? []
  const own = new Set(children)
  // A current partner's children who aren't this person's own.
  const stepChildren = partners
    .filter((p) => !p.ended)
    .flatMap((p) => (childrenOf.get(p.id) ?? []).filter((c) => !own.has(c)).map((c) => ({ id: c, via: p.id })))
  return {
    parents: parentsOf.get(personId) ?? [],
    partners: partners.sort((a, b) => Number(a.ended) - Number(b.ended)),
    children,
    stepChildren: stepChildren.filter((s, i) => stepChildren.findIndex((t) => t.id === s.id) === i),
    grandchildren: [...new Set(children.flatMap((c) => childrenOf.get(c) ?? []))],
  }
}
