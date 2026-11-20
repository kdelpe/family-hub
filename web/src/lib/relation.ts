import type { Tree, TreePerson } from '../api/tree'

type Gender = TreePerson['gender']
type Couple = { partner: string; married: boolean; ended: boolean }
type Kin = { up: number; down: number }

export const gendered = (gender: Gender, male: string, female: string, other: string) =>
  gender === 'male' ? male : gender === 'female' ? female : other

const greats = (n: number) => 'great-'.repeat(Math.max(0, n))
const former = (ended: boolean, word: string) => (ended ? `former ${word}` : word)

const add = <T,>(map: Map<string, T[]>, key: string, value: T) => {
  const list = map.get(key)
  if (list) {
    list.push(value)
  } else {
    map.set(key, [value])
  }
}

// How each person in the tree is related to the viewer, as their card says it: "You", "Your half-sister",
// "Your great-aunt". Derived only from parent links and couples, by the rules of architecture §3.4. Null for
// someone the tree doesn't connect to the viewer.
export function relationsTo(tree: Tree, viewerId: string): Map<string, string> {
  const gender = new Map(tree.persons.map((p) => [p.id, p.gender]))
  const parentLists = new Map<string, string[]>()
  const childLists = new Map<string, string[]>()
  const fosterParentLists = new Map<string, string[]>()
  const fosterChildLists = new Map<string, string[]>()
  const couples = new Map<string, Couple[]>()
  for (const r of tree.relationships) {
    if (r.type === 'parent_child') {
      // Foster links never make family (§3.4): they only give the foster labels.
      if (r.subtype === 'foster') {
        add(fosterParentLists, r.child_id, r.parent_id)
        add(fosterChildLists, r.parent_id, r.child_id)
      } else {
        add(parentLists, r.child_id, r.parent_id)
        add(childLists, r.parent_id, r.child_id)
      }
    } else {
      const [a, b] = r.person_ids
      const married = r.subtype === 'married'
      add(couples, a, { partner: b, married, ended: r.ended })
      add(couples, b, { partner: a, married, ended: r.ended })
    }
  }
  const setOf = (map: Map<string, string[]>, id: string) => new Set(map.get(id) ?? [])
  const parentsOf = (id: string) => setOf(parentLists, id)
  const fosterParentsOf = (id: string) => setOf(fosterParentLists, id)
  // Current couples first, so a current tie wins over an ended one.
  const partnersOf = (id: string) => [...(couples.get(id) ?? [])].sort((x, y) => Number(x.ended) - Number(y.ended))

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
    const fromB = ancestors(b)
    let best: Kin | null = null
    for (const [id, up] of ancestors(a)) {
      const down = fromB.get(id)
      if (down !== undefined && (!best || up + down < best.up + best.down)) {
        best = { up, down }
      }
    }
    return best
  }

  // Full needs the same two or more parents; when the recorded parents prove neither full nor half, the
  // neutral word, rather than a guess (§3.4).
  const siblingWord = (a: string, b: string, g: Gender) => {
    const pa = parentsOf(a)
    const pb = parentsOf(b)
    const aOnly = [...pa].some((p) => !pb.has(p))
    const bOnly = [...pb].some((p) => !pa.has(p))
    if (aOnly && bOnly) {
      return gendered(g, 'half-brother', 'half-sister', 'half-sibling')
    }
    return !aOnly && !bOnly && pa.size >= 2 ? gendered(g, 'brother', 'sister', 'sibling') : 'sibling'
  }

  // The word for someone of gender g standing where b stands from a. In-laws don't tell full from half.
  const kinWord = (a: string, b: string, { up, down }: Kin, g: Gender, inLaw = false): string | null => {
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
      return inLaw ? gendered(g, 'brother', 'sister', 'sibling') : siblingWord(a, b, g)
    }
    if (down === 1) {
      return `${greats(up - 2)}${gendered(g, 'uncle', 'aunt', 'aunt or uncle')}`
    }
    if (up === 1) {
      return `${greats(down - 2)}${gendered(g, 'nephew', 'niece', 'niece or nephew')}`
    }
    return 'cousin'
  }

  // A family parent's partner who isn't the child's parent by any link (§3.4).
  const stepCouple = (child: string, person: string) =>
    parentsOf(child).has(person) || fosterParentsOf(child).has(person)
      ? undefined
      : [...parentsOf(child)].flatMap((p) => partnersOf(p)).find((c) => c.partner === person)

  const viewerParents = parentsOf(viewerId)
  const viewerFoster = fosterParentsOf(viewerId)
  const viewerStepParents = [...viewerParents]
    .flatMap((p) => partnersOf(p))
    .filter((c) => stepCouple(viewerId, c.partner))

  const word = (id: string): string | null => {
    const g = gender.get(id) ?? null

    const couple = partnersOf(viewerId).find((c) => c.partner === id)
    if (couple) {
      return former(couple.ended, couple.married ? gendered(g, 'husband', 'wife', 'spouse') : 'partner')
    }

    const blood = kin(viewerId, id)
    if (blood) {
      return kinWord(viewerId, id, blood, g)
    }

    const asStepParent = stepCouple(viewerId, id)
    if (asStepParent) {
      return former(asStepParent.ended, gendered(g, 'stepfather', 'stepmother', 'step-parent'))
    }
    const asStepChild = stepCouple(id, viewerId)
    if (asStepChild) {
      return former(asStepChild.ended, gendered(g, 'stepson', 'stepdaughter', 'stepchild'))
    }
    // A step-parent's child; sharing a parent would have made them a sibling above.
    const viaStepParent = viewerStepParents.find((c) => parentsOf(id).has(c.partner))
    if (viaStepParent) {
      return former(viaStepParent.ended, gendered(g, 'stepbrother', 'stepsister', 'step-sibling'))
    }

    if (viewerFoster.has(id)) {
      return gendered(g, 'foster father', 'foster mother', 'foster parent')
    }
    if (fosterParentsOf(id).has(viewerId)) {
      return gendered(g, 'foster son', 'foster daughter', 'foster child')
    }
    const theirFoster = fosterParentsOf(id)
    if (
      [...viewerFoster].some((p) => theirFoster.has(p) || parentsOf(id).has(p)) ||
      [...theirFoster].some((p) => viewerParents.has(p))
    ) {
      return gendered(g, 'foster brother', 'foster sister', 'foster sibling')
    }

    // In-laws: one couple hop from the above (§3.4). First your partner's family…
    for (const { partner, ended } of partnersOf(viewerId)) {
      const k = kin(partner, id)
      const w = k && kinWord(partner, id, k, g, true)
      if (k && w) {
        // Your partner's child or grandchild who isn't yours is a step-relation, not an in-law.
        return former(ended, k.up === 0 ? `step${k.down === 1 ? '' : '-'}${w}` : `${w}-in-law`)
      }
    }
    // …then your relatives' partners.
    for (const { partner, ended } of partnersOf(id)) {
      const k = kin(viewerId, partner)
      const w = k && kinWord(viewerId, partner, k, g, true)
      if (k && w) {
        // Your aunt's husband is your uncle; your grandmother's husband, a step-grandfather.
        if (k.down === 1 && k.up >= 2) {
          return former(ended, w)
        }
        return former(ended, k.down === 0 ? `step-${w}` : `${w}-in-law`)
      }
    }
    return null
  }

  const labels = new Map<string, string>()
  for (const p of tree.persons) {
    if (p.id === viewerId) {
      labels.set(p.id, 'You')
      continue
    }
    const w = word(p.id)
    if (w !== null) {
      labels.set(p.id, `Your ${w}`)
    }
  }
  return labels
}
