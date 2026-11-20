import type { Tree, TreePerson } from '../api/tree'

type Gender = TreePerson['gender']
type Couple = { partner: string; married: boolean; ended: boolean }
type Kin = { up: number; down: number }

const add = (map: Map<string, Set<string>>, key: string, value: string) => {
  const set = map.get(key) ?? new Set<string>()
  set.add(value)
  map.set(key, set)
}

const gendered = (gender: Gender, male: string, female: string, other: string) =>
  gender === 'male' ? male : gender === 'female' ? female : other

const greats = (n: number) => 'great-'.repeat(Math.max(0, n))

// How each person in the tree is related to the viewer, as their card says it: "You", "Your half-sister",
// "Your great-aunt". Derived only from parent links and couples, by the rules of architecture §3.4. Null for
// someone the tree doesn't connect to the viewer.
export function relationsTo(tree: Tree, viewerId: string): (personId: string) => string | null {
  const gender = new Map(tree.persons.map((p) => [p.id, p.gender]))
  const parents = new Map<string, Set<string>>()
  const children = new Map<string, Set<string>>()
  const fosterParents = new Map<string, Set<string>>()
  const fosterChildren = new Map<string, Set<string>>()
  const couples = new Map<string, Couple[]>()
  for (const r of tree.relationships) {
    if (r.type === 'parent_child') {
      // Foster links never make family (§3.4): they only give the foster labels.
      if (r.subtype === 'foster') {
        add(fosterParents, r.child_id, r.parent_id)
        add(fosterChildren, r.parent_id, r.child_id)
      } else {
        add(parents, r.child_id, r.parent_id)
        add(children, r.parent_id, r.child_id)
      }
    } else {
      const [a, b] = r.person_ids
      const married = r.subtype === 'married'
      couples.set(a, [...(couples.get(a) ?? []), { partner: b, married, ended: r.ended }])
      couples.set(b, [...(couples.get(b) ?? []), { partner: a, married, ended: r.ended }])
    }
  }
  const parentsOf = (id: string) => parents.get(id) ?? new Set<string>()
  const childrenOf = (id: string) => children.get(id) ?? new Set<string>()
  const partnersOf = (id: string) => couples.get(id) ?? []

  const ancestorCache = new Map<string, Map<string, number>>()
  // Each ancestor with its fewest generations up, the person themself at 0.
  const ancestors = (id: string) => {
    const cached = ancestorCache.get(id)
    if (cached) {
      return cached
    }
    const depth = new Map([[id, 0]])
    const queue = [id]
    for (let i = 0; i < queue.length; i++) {
      const at = queue[i]!
      for (const parent of parentsOf(at)) {
        if (!depth.has(parent)) {
          depth.set(parent, depth.get(at)! + 1)
          queue.push(parent)
        }
      }
    }
    ancestorCache.set(id, depth)
    return depth
  }

  // Blood (or adoptive) kinship through the closest common ancestor: generations up from a, then down to b.
  const kin = (a: string, b: string): Kin | null => {
    const fromA = ancestors(a)
    const fromB = ancestors(b)
    let best: Kin | null = null
    for (const [id, up] of fromA) {
      const down = fromB.get(id)
      if (down !== undefined && (!best || up + down < best.up + best.down)) {
        best = { up, down }
      }
    }
    return best
  }

  const siblingWord = (a: string, b: string) => {
    const pa = parentsOf(a)
    const pb = parentsOf(b)
    const aOnly = [...pa].some((p) => !pb.has(p))
    const bOnly = [...pb].some((p) => !pa.has(p))
    const g = gender.get(b) ?? null
    return aOnly && bOnly ? gendered(g, 'half-brother', 'half-sister', 'half-sibling') : gendered(g, 'brother', 'sister', 'sibling')
  }

  // The word for b as seen from a, given their kinship.
  const kinWord = (a: string, b: string, { up, down }: Kin): string | null => {
    const g = gender.get(b) ?? null
    if (up === 0 && down === 0) {
      return null
    }
    if (up === 0) {
      return down === 1
        ? gendered(g, 'son', 'daughter', 'child')
        : `${greats(down - 2)}${gendered(g, 'grandson', 'granddaughter', 'grandchild')}`
    }
    if (down === 0) {
      return up === 1
        ? gendered(g, 'father', 'mother', 'parent')
        : `${greats(up - 2)}${gendered(g, 'grandfather', 'grandmother', 'grandparent')}`
    }
    if (up === 1 && down === 1) {
      return siblingWord(a, b)
    }
    if (down === 1) {
      return `${greats(up - 2)}${gendered(g, 'uncle', 'aunt', 'aunt or uncle')}`
    }
    if (up === 1) {
      return `${greats(down - 2)}${gendered(g, 'nephew', 'niece', 'niece or nephew')}`
    }
    return 'cousin'
  }

  const stepParent = (child: string, person: string) =>
    [...parentsOf(child)]
      .flatMap((p) => partnersOf(p))
      .find((c) => c.partner === person && !parentsOf(child).has(person) && !fosterParents.get(child)?.has(person))

  const word = (id: string): string | null => {
    const g = gender.get(id) ?? null

    const couple = partnersOf(viewerId)
      .filter((c) => c.partner === id)
      .sort((x, y) => Number(x.ended) - Number(y.ended))[0]
    if (couple) {
      const w = couple.married ? gendered(g, 'husband', 'wife', 'spouse') : 'partner'
      return couple.ended ? `former ${w}` : w
    }

    const blood = kin(viewerId, id)
    if (blood) {
      return kinWord(viewerId, id, blood)
    }

    const asStepParent = stepParent(viewerId, id)
    if (asStepParent) {
      return `${asStepParent.ended ? 'former ' : ''}${gendered(g, 'stepfather', 'stepmother', 'step-parent')}`
    }
    const asStepChild = stepParent(id, viewerId)
    if (asStepChild) {
      return `${asStepChild.ended ? 'former ' : ''}${gendered(g, 'stepson', 'stepdaughter', 'stepchild')}`
    }
    // A step-parent's child; sharing a parent would have made them a sibling above.
    const stepParents = [...parentsOf(viewerId)]
      .flatMap((p) => partnersOf(p).map((c) => c.partner))
      .filter((q) => stepParent(viewerId, q))
    if (stepParents.some((q) => childrenOf(q).has(id))) {
      return gendered(g, 'stepbrother', 'stepsister', 'step-sibling')
    }

    if (fosterParents.get(viewerId)?.has(id)) {
      return gendered(g, 'foster father', 'foster mother', 'foster parent')
    }
    if (fosterChildren.get(viewerId)?.has(id)) {
      return gendered(g, 'foster son', 'foster daughter', 'foster child')
    }
    const viewerFoster = fosterParents.get(viewerId) ?? new Set<string>()
    const theirFoster = fosterParents.get(id) ?? new Set<string>()
    if (
      [...viewerFoster].some((p) => theirFoster.has(p)) ||
      [...viewerFoster].some((p) => parentsOf(id).has(p)) ||
      [...theirFoster].some((p) => parentsOf(viewerId).has(p))
    ) {
      return gendered(g, 'foster brother', 'foster sister', 'foster sibling')
    }

    // In-laws: one hop over a current couple (§3.4).
    for (const { partner, ended } of partnersOf(viewerId)) {
      const k = !ended && kin(partner, id)
      if (k && k.up === 1 && k.down === 0) {
        return gendered(g, 'father-in-law', 'mother-in-law', 'parent-in-law')
      }
      if (k && k.up === 1 && k.down === 1) {
        return gendered(g, 'brother-in-law', 'sister-in-law', 'sibling-in-law')
      }
    }
    for (const { partner, ended } of partnersOf(id)) {
      const k = !ended && kin(viewerId, partner)
      if (k && k.up === 0 && k.down === 1) {
        return gendered(g, 'son-in-law', 'daughter-in-law', 'child-in-law')
      }
      if (k && k.up === 1 && k.down === 1) {
        return gendered(g, 'brother-in-law', 'sister-in-law', 'sibling-in-law')
      }
      // An aunt's husband is an uncle too.
      if (k && k.up >= 2 && k.down === 1) {
        return `${greats(k.up - 2)}${gendered(g, 'uncle', 'aunt', 'aunt or uncle')}`
      }
    }
    return null
  }

  return (personId: string) => {
    if (personId === viewerId) {
      return 'You'
    }
    const w = word(personId)
    return w === null ? null : `Your ${w}`
  }
}
