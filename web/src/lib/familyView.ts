import type { FamilyLinks } from './familyLinks'

// The tree shows foster links too, labelled (architecture §7), so here a parent is any parent link.
export const treeParentsOf = (links: FamilyLinks, id: string) => [...links.parentsOf(id), ...links.fosterParentsOf(id)]
export const treeChildrenOf = (links: FamilyLinks, id: string) => [...links.childrenOf(id), ...links.fosterChildrenOf(id)]

export type FocusView = {
  // Each of the focus person's parents with that parent's own parents.
  grandparents: { of: string; ids: string[] }[]
  parents: string[]
  // Foster: linked only through a foster parent, on either side.
  siblings: { id: string; half: boolean; foster: boolean }[]
  partners: string[]
  formerPartners: string[]
  // The focus person's children, one group per other parent, so each marriage shows as its own household.
  children: { with: string[]; ids: string[] }[]
  grandchildren: { of: string; ids: string[] }[]
}

// The generations around one person (architecture §7, design-notes §4.2). Each person appears once, in the
// first place met, even where the family loops back on itself.
export function focusView(links: FamilyLinks, focus: string): FocusView {
  const parentsOf = (id: string) => treeParentsOf(links, id)
  const childrenOf = (id: string) => treeChildrenOf(links, id)
  const shown = new Set<string>()
  const fresh = (ids: string[]) => {
    const out = [...new Set(ids)].filter((id) => !shown.has(id))
    out.forEach((id) => shown.add(id))
    return out
  }
  const couples = links.partnersOf(focus)
  shown.add(focus)
  const partners = fresh(couples.filter((p) => !p.ended).map((p) => p.id))
  const formerPartners = fresh(couples.filter((p) => p.ended).map((p) => p.id))
  const parents = fresh(parentsOf(focus))
  const grandparents = parents.map((of) => ({ of, ids: fresh(parentsOf(of)) })).filter((g) => g.ids.length > 0)
  const mine = links.parentsOf(focus)
  const siblings = fresh(parents.flatMap(childrenOf)).map((id) => ({
    id,
    half: links.halfSiblings(focus, id),
    foster: ![...links.parentsOf(id)].some((p) => mine.has(p)),
  }))
  // One group per other family parent: a foster link doesn't split a child from their household, and the
  // focus person's own foster children sit in the group with no other parent.
  const byOtherParent = new Map<string, string[]>()
  for (const child of fresh(childrenOf(focus))) {
    const key = links.parentsOf(child).has(focus)
      ? [...links.parentsOf(child)]
          .filter((p) => p !== focus)
          .sort()
          .join(' ')
      : ''
    const group = byOtherParent.get(key)
    if (group) {
      group.push(child)
    } else {
      byOtherParent.set(key, [child])
    }
  }
  const children = [...byOtherParent].map(([key, ids]) => ({ with: key ? key.split(' ') : [], ids }))
  const grandchildren = children
    .flatMap((g) => g.ids)
    .map((of) => ({ of, ids: fresh(childrenOf(of)) }))
    .filter((g) => g.ids.length > 0)
  return { grandparents, parents, siblings, partners, formerPartners, children, grandchildren }
}
