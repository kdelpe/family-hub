import type { Tree, TreePerson } from '../api/tree'

type Gender = TreePerson['gender']
type Couple = { partner: string; married: boolean; ended: boolean }
type Kin = { up: number; down: number }

export const gendered = (gender: Gender, male: string, female: string, other: string) =>
  gender === 'male' ? male : gender === 'female' ? female : other

const greats = (n: number) => 'great-'.repeat(Math.max(0, n))

const ordinals = ['', 'second ', 'third ', 'fourth ', 'fifth ']
const ordinal = (n: number) => {
  const suffix = n % 100 >= 11 && n % 100 <= 13 ? 'th' : (['th', 'st', 'nd', 'rd'][n % 10] ?? 'th')
  return `${n}${suffix} `
}
const removals = ['', ' once removed', ' twice removed']

// "Cousin", "second cousin", "cousin once removed": the degree from the nearer generation, removed by the gap.
const cousinWord = (up: number, down: number) => {
  const degree = Math.min(up, down) - 1
  const gap = Math.abs(up - down)
  return `${ordinals[degree - 1] ?? ordinal(degree)}cousin${removals[gap] ?? ` ${gap} times removed`}`
}
const former = (ended: boolean, word: string) => (ended ? `former ${word}` : word)
const inLaw = (word: string) => word.split(' or ').map((w) => `${w}-in-law`).join(' or ')

const add = <T,>(map: Map<string, T[]>, key: string, value: T) => {
  const list = map.get(key)
  if (list) {
    list.push(value)
  } else {
    map.set(key, [value])
  }
}

// Each person's relation to the viewer ("You", "Your half-sister"), from parent links and couples by §3.4;
// someone the tree doesn't connect to the viewer has no entry.
export function relationsTo(tree: Tree, viewerId: string): Map<string, string> {
  const gender = new Map(tree.persons.map((p) => [p.id, p.gender]))
  const parentLists = new Map<string, string[]>()
  const fosterParentLists = new Map<string, string[]>()
  const couples = new Map<string, Couple[]>()
  for (const r of tree.relationships) {
    if (r.type === 'parent_child') {
      // Foster links never make family (§3.4): they only give the foster labels.
      if (r.subtype === 'foster') {
        add(fosterParentLists, r.child_id, r.parent_id)
      } else {
        add(parentLists, r.child_id, r.parent_id)
      }
    } else {
      const [a, b] = r.person_ids
      const married = r.subtype === 'married'
      add(couples, a, { partner: b, married, ended: r.ended })
      add(couples, b, { partner: a, married, ended: r.ended })
    }
  }
  const none = new Set<string>()
  const parentSets = new Map([...parentLists].map(([id, list]) => [id, new Set(list)]))
  const fosterParentSets = new Map([...fosterParentLists].map(([id, list]) => [id, new Set(list)]))
  // Current couples first, so a pair who split and got back together reads as current.
  for (const list of couples.values()) {
    list.sort((x, y) => Number(x.ended) - Number(y.ended))
  }
  const parentsOf = (id: string) => parentSets.get(id) ?? none
  const fosterParentsOf = (id: string) => fosterParentSets.get(id) ?? none
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
      const gr = greats(up - 2)
      return gendered(g, `${gr}uncle`, `${gr}aunt`, `${gr}aunt or ${gr}uncle`)
    }
    if (up === 1) {
      const gr = greats(down - 2)
      return gendered(g, `${gr}nephew`, `${gr}niece`, `${gr}niece or ${gr}nephew`)
    }
    return cousinWord(up, down)
  }

  // A family parent's partner who isn't the child's parent by any link (§3.4).
  const stepCouple = (child: string, person: string) =>
    parentsOf(child).has(person) || fosterParentsOf(child).has(person)
      ? undefined
      : [...parentsOf(child)]
          .flatMap((p) => partnersOf(p))
          .filter((c) => c.partner === person)
          .sort((x, y) => Number(x.ended) - Number(y.ended))[0]

  const viewerParents = parentsOf(viewerId)
  const viewerFoster = fosterParentsOf(viewerId)
  // Step-siblings come only through a current couple: a former step-parent's later children were never family.
  const viewerStepParents = [...viewerParents]
    .flatMap((p) => partnersOf(p))
    .filter((c) => !c.ended && stepCouple(viewerId, c.partner))

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
      return gendered(g, 'stepbrother', 'stepsister', 'step-sibling')
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

    // In-laws: one couple hop from the above (§3.4), current couples before ended ones on either side.
    for (const pass of [false, true]) {
      // Your partner's family: their niece is your niece; their grandchild who isn't yours, a step-grandchild.
      for (const { partner, ended } of partnersOf(viewerId).filter((c) => c.ended === pass)) {
        const k = kin(partner, id)
        const w = k && kinWord(partner, id, k, g, true)
        if (k && w) {
          if (k.up === 1 && k.down >= 2) {
            return former(ended, w)
          }
          return former(ended, k.up === 0 ? `step-${w}` : inLaw(w))
        }
      }
      // Your relatives' partners: your aunt's husband is your uncle; your grandmother's, a step-grandfather.
      for (const { partner, ended } of partnersOf(id).filter((c) => c.ended === pass)) {
        const k = kin(viewerId, partner)
        const w = k && kinWord(viewerId, partner, k, g, true)
        if (k && w) {
          if (k.down === 1 && k.up >= 2) {
            return former(ended, w)
          }
          return former(ended, k.down === 0 ? `step-${w}` : inLaw(w))
        }
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
