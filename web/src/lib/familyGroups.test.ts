import { expect, it } from 'vitest'

import type { Tree, TreeRelationship } from '../api/tree'
import { familyGroups } from './familyGroups'

let n = 0
const parent = (parent_id: string, child_id: string, subtype: 'biological' | 'foster' = 'biological'): TreeRelationship => ({
  id: `r${n++}`,
  type: 'parent_child',
  subtype,
  parent_id,
  child_id,
})

const tree: Tree = {
  persons: [],
  relationships: [
    parent('lucien', 'nadege'),
    parent('rose', 'nadege'),
    { id: 'c1', type: 'couple', subtype: 'married', ended: false, person_ids: ['jeanrobert', 'nadege'] },
    { id: 'c2', type: 'couple', subtype: 'partner', ended: true, person_ids: ['nadege', 'paul'] },
    parent('nadege', 'maya'),
    parent('jeanrobert', 'maya'),
    parent('jeanrobert', 'sabine'),
    parent('paul', 'lise'),
    parent('maya', 'naika'),
    parent('nadege', 'lucie', 'foster'),
    parent('jeanrobert', 'lucie'),
    { id: 'c3', type: 'couple', subtype: 'married', ended: true, person_ids: ['nadege', 'jeanrobert'] },
  ],
}

it('groups parents, partners, children, step-children and grandchildren', () => {
  expect(familyGroups(tree, 'nadege')).toEqual({
    parents: ['lucien', 'rose'],
    partners: [
      { id: 'jeanrobert', married: true, ended: false },
      { id: 'paul', married: false, ended: true },
    ],
    children: ['maya'],
    // Only a current partner's children, never Nadège's own or foster ones; the earlier split doesn't list him twice.
    stepChildren: [{ id: 'sabine', via: 'jeanrobert' }],
    grandchildren: ['naika'],
  })
})
