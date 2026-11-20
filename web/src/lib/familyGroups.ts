import type { Tree } from '../api/tree'
import { type FamilyLinks, familyLinks, type Partner } from './familyLinks'
export type StepChild = { id: string; via: string }

export type FamilyGroups = {
  parents: string[]
  partners: Partner[]
  children: string[]
  stepChildren: StepChild[]
  grandchildren: string[]
}

// One person's close family for their profile (design-notes §4.9), over family-parent links (§3.4).
export function familyGroups(tree: Tree, personId: string, links: FamilyLinks = familyLinks(tree)): FamilyGroups {
  const { parentsOf, childrenOf, fosterChildrenOf, partnersOf } = links
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
    parents: [...parentsOf(personId)],
    partners,
    children,
    stepChildren: [...stepChildren.values()],
    grandchildren: [...new Set(children.flatMap(childrenOf))],
  }
}
