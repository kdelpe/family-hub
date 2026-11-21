import type { Tree, TreeRelationship } from '../api/tree'

export type ParentLink = Extract<TreeRelationship, { type: 'parent_child' }>['subtype']
export type Partner = { id: string; married: boolean; ended: boolean }

export type FamilyLinks = {
  // Family parents and children: biological and adoptive. Foster links never make family (§3.4).
  parentsOf: (id: string) => ReadonlySet<string>
  childrenOf: (id: string) => string[]
  fosterParentsOf: (id: string) => ReadonlySet<string>
  fosterChildrenOf: (id: string) => string[]
  // One entry per partner, current couples first; a pair who split and got back together counts as current.
  partnersOf: (id: string) => Partner[]
  linkOf: (child: string, parent: string) => ParentLink | undefined
  // They share a family parent, and each has one the other doesn't.
  halfSiblings: (a: string, b: string) => boolean
}

// Who links to whom in the tree, looked up by person: the one place the pages' family rules start from.
export function familyLinks(tree: Tree): FamilyLinks {
  const parents = new Map<string, Set<string>>()
  const children = new Map<string, string[]>()
  const fosterParents = new Map<string, Set<string>>()
  const fosterChildren = new Map<string, string[]>()
  const links = new Map<string, ParentLink>()
  const couples = new Map<string, Map<string, Partner>>()
  const addTo = (map: Map<string, Set<string>>, key: string, value: string) => {
    const set = map.get(key) ?? new Set<string>()
    set.add(value)
    map.set(key, set)
  }
  const push = (map: Map<string, string[]>, key: string, value: string) => {
    const list = map.get(key)
    if (list) {
      list.push(value)
    } else {
      map.set(key, [value])
    }
  }
  for (const r of tree.relationships) {
    if (r.type === 'parent_child') {
      links.set(`${r.child_id} ${r.parent_id}`, r.subtype)
      if (r.subtype === 'foster') {
        addTo(fosterParents, r.child_id, r.parent_id)
        push(fosterChildren, r.parent_id, r.child_id)
      } else {
        addTo(parents, r.child_id, r.parent_id)
        push(children, r.parent_id, r.child_id)
      }
    } else {
      const [a, b] = r.person_ids
      for (const [x, y] of [
        [a, b],
        [b, a],
      ] as const) {
        const theirs = couples.get(x) ?? new Map<string, Partner>()
        const known = theirs.get(y)
        if (!known || (known.ended && !r.ended)) {
          theirs.set(y, { id: y, married: r.subtype === 'married', ended: r.ended })
        }
        couples.set(x, theirs)
      }
    }
  }
  const none: ReadonlySet<string> = new Set()
  const partners = new Map([...couples].map(([id, m]) => [id, [...m.values()].sort((x, y) => Number(x.ended) - Number(y.ended))]))
  const parentsOf = (id: string) => parents.get(id) ?? none
  return {
    parentsOf,
    childrenOf: (id) => children.get(id) ?? [],
    fosterParentsOf: (id) => fosterParents.get(id) ?? none,
    fosterChildrenOf: (id) => fosterChildren.get(id) ?? [],
    partnersOf: (id) => partners.get(id) ?? [],
    linkOf: (child, parent) => links.get(`${child} ${parent}`),
    halfSiblings: (a, b) => {
      const pa = [...parentsOf(a)]
      const pb = parentsOf(b)
      return pa.some((p) => pb.has(p)) && pa.some((p) => !pb.has(p)) && [...pb].some((p) => !pa.includes(p))
    },
  }
}
