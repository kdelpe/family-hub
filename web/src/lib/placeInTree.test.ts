import { expect, it } from 'vitest'

import type { Tree, TreePerson } from '../api/tree'
import { placeInTree } from './placeInTree'

const person = (id: string, first_name: string, gender: TreePerson['gender']): TreePerson => ({
  id,
  first_name,
  last_name: 'Pluviose',
  gender,
  is_deceased: false,
})

const tree: Tree = {
  persons: [
    person('fritz', 'Fritz', 'male'),
    person('fabienne', 'Fabienne', 'female'),
    person('kervens', 'Kervens', 'male'),
    person('esther', 'Esther', 'female'),
    person('jonas', 'Jonas', null),
    person('peterson', 'Peterson', 'male'),
  ],
  relationships: [
    { id: '1', type: 'parent_child', parent_id: 'fritz', child_id: 'kervens' },
    { id: '2', type: 'parent_child', parent_id: 'fabienne', child_id: 'kervens' },
    { id: '3', type: 'parent_child', parent_id: 'fabienne', child_id: 'esther' },
    { id: '4', type: 'parent_child', parent_id: 'fritz', child_id: 'jonas' },
    { id: '5', type: 'couple', ended: false, person_ids: ['fabienne', 'fritz'] },
  ],
}

it.each([
  { id: 'kervens', want: 'In the tree as Fritz and Fabienne’s son' },
  { id: 'esther', want: 'In the tree as Fabienne’s daughter' },
  { id: 'jonas', want: 'In the tree as Fritz’s child' },
  { id: 'peterson', want: 'In the tree' },
  { id: 'nobody', want: 'In the tree' },
])('$id: $want', ({ id, want }) => {
  expect(placeInTree(tree, id)).toBe(want)
})
