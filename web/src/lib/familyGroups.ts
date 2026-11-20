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
  const fostered = new Set<string>()
  const partners = new Map<string, Partner>()
  for (const r of tree.relationships) {
    if (r.type === 'parent_child' && r.subtype === 'foster') {
      if (r.parent_id === personId) {
        fostered.add(r.child_id)
      }
    } else if (r.type === 'parent_child') {
      parentsOf.set(r.child_id, [...(parentsOf.get(r.child_id) ?? []), r.parent_id])
      childrenOf.set(r.parent_id, [...(childrenOf.get(r.parent_id) ?? []), r.child_id])
    } else if (r.person_ids.includes(personId)) {
      const id = r.person_ids[0] === personId ? r.person_ids[1] : r.person_ids[0]
      const known = partners.get(id)
      // A pair who split and got back together reads as current, as in relation.ts.
      if (!known || (known.ended && !r.ended)) {
        partners.set(id, { id, married: r.subtype === 'married', ended: r.ended })
      }
    }
  }
  const children = childrenOf.get(personId) ?? []
  // Never a step-parent to one's own or foster child.
  const notStep = new Set([...children, ...fostered])
  const stepChildren = new Map<string, StepChild>()
  for (const p of partners.values()) {
    if (p.ended) {
      continue
    }
    for (const c of childrenOf.get(p.id) ?? []) {
      if (!notStep.has(c) && !stepChildren.has(c)) {
        stepChildren.set(c, { id: c, via: p.id })
      }
    }
  }
  return {
    parents: parentsOf.get(personId) ?? [],
    partners: [...partners.values()],
    children,
    stepChildren: [...stepChildren.values()],
    grandchildren: [...new Set(children.flatMap((c) => childrenOf.get(c) ?? []))],
  }
}
