import { describe, expect, it } from 'vitest'

import type { Tree, TreeRelationship } from '../api/tree'
import { familyLinks } from './familyLinks'

let n = 0
const parent = (parent_id: string, child_id: string, subtype: 'biological' | 'adoptive' | 'foster' = 'biological'): TreeRelationship => ({
  id: `r${n++}`,
  type: 'parent_child',
  subtype,
  parent_id,
  child_id,
})
const couple = (a: string, b: string, ended: boolean, married = true): TreeRelationship => ({
  id: `r${n++}`,
  type: 'couple',
  subtype: married ? 'married' : 'partner',
  ended,
  person_ids: [a, b],
})

// Maya and Fritz share both parents; Sabine shares only Jean-Robert; Odette shares one parent and has no other.
// Tim is Maya's adopted son, Lise her foster daughter. Maya and Stanley split and got back together.
const tree: Tree = {
  persons: [],
  relationships: [
    ...['maya', 'fritz'].flatMap((c) => [parent('jeanrobert', c), parent('nadege', c)]),
    parent('jeanrobert', 'sabine'),
    parent('guerline', 'sabine'),
    parent('jeanrobert', 'odette'),
    parent('maya', 'tim', 'adoptive'),
    parent('maya', 'lise', 'foster'),
    couple('maya', 'stanley', true, false),
    couple('maya', 'stanley', false),
    couple('maya', 'yves', true),
  ],
}
const links = familyLinks(tree)

describe('familyLinks', () => {
  it('keeps foster links apart from family links', () => {
    expect(links.childrenOf('maya')).toEqual(['tim'])
    expect(links.fosterChildrenOf('maya')).toEqual(['lise'])
    expect([...links.fosterParentsOf('lise')]).toEqual(['maya'])
    expect(links.parentsOf('lise').size).toBe(0)
    expect(links.linkOf('tim', 'maya')).toBe('adoptive')
    expect(links.linkOf('fritz', 'maya')).toBeUndefined()
  })

  it('lists each partner once, current first, a reunited couple as current', () => {
    expect(links.partnersOf('maya')).toEqual([
      { id: 'stanley', married: true, ended: false },
      { id: 'yves', married: true, ended: true },
    ])
  })

  it.each([
    { name: 'same two parents', a: 'maya', b: 'fritz', half: false },
    { name: 'one parent each the other lacks', a: 'maya', b: 'sabine', half: true },
    { name: 'only one parent recorded', a: 'maya', b: 'odette', half: false },
    { name: 'no shared parent', a: 'maya', b: 'lise', half: false },
  ])('half-siblings: $name', ({ a, b, half }) => {
    expect(links.halfSiblings(a, b)).toBe(half)
    expect(links.halfSiblings(b, a)).toBe(half)
  })
})
