import type { Tree } from '../api/tree'
import { type FamilyLinks, familyLinks, type Partner } from './familyLinks'
export type StepChild = { id: string; via: string }

export type FamilyGroups = {
  parents: string[]
  fosterParents: string[]
  siblings: string[]
  partners: Partner[]
  children: string[]
  fosterChildren: string[]
  stepChildren: StepChild[]
  grandchildren: string[]
}

// One person's close family for their profile (design-notes §4.9), over family-parent links (§3.4).
export function familyGroups(tree: Tree, personId: string, links: FamilyLinks = familyLinks(tree)): FamilyGroups {
  const { parentsOf, childrenOf, fosterParentsOf, fosterChildrenOf, partnersOf } = links
  const parents = [...parentsOf(personId)]
  const children = childrenOf(personId)
  const partners = partnersOf(personId)
  // Never a step-parent to one's own or foster child.
  const notStep = new Set([...children, ...fosterChildrenOf(personId)])
  const stepChildren = new Map<string, StepChild>()
  for (const p of partners) {
    if (p.ended) {
      continue
    }
    for (const c of childrenOf(p.id)) {
      if (!notStep.has(c) && !stepChildren.has(c)) {
        stepChildren.set(c, { id: c, via: p.id })
      }
    }
  }
  return {
    parents,
    fosterParents: [...fosterParentsOf(personId)],
    siblings: [...new Set(parents.flatMap(childrenOf))].filter((id) => id !== personId),
    partners,
    children,
    fosterChildren: fosterChildrenOf(personId),
    stepChildren: [...stepChildren.values()],
    grandchildren: [...new Set(children.flatMap(childrenOf))],
  }
}
