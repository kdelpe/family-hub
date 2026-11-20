import { describe, expect, it } from 'vitest'

import type { Tree, TreePerson, TreeRelationship } from '../api/tree'
import { relationsTo } from './relation'

const person = (id: string, gender: TreePerson['gender']): TreePerson => ({
  id,
  first_name: id,
  last_name: null,
  side: null,
  gender,
  is_deceased: false,
})

let n = 0
const parent = (parent_id: string, child_id: string, subtype: 'biological' | 'adoptive' | 'foster' = 'biological'): TreeRelationship => ({
  id: `r${n++}`,
  type: 'parent_child',
  subtype,
  parent_id,
  child_id,
})
const couple = (a: string, b: string, { married = true, ended = false } = {}): TreeRelationship => ({
  id: `r${n++}`,
  type: 'couple',
  subtype: married ? 'married' : 'partner',
  ended,
  person_ids: [a, b],
})
const parents = (child: string, ...ps: string[]) => ps.map((p) => parent(p, child))

// Maya's family, drawn so every label has a case.
const tree: Tree = {
  persons: [
    ...['jeanbaptiste', 'joseph', 'jeanrobert', 'fritz', 'kervens', 'tim', 'mackenson', 'lucien', 'frantz', 'stanley', 'wesner', 'ralph', 'henri', 'pierre', 'paul', 'xavier'].map((id) => person(id, 'male')),
    ...['annemarie', 'marieclaire', 'marietherese', 'nadege', 'maya', 'sabine', 'guerline', 'fabienne', 'naika', 'naomie', 'rose', 'mirlande', 'darline', 'odette', 'lise', 'lucie'].map((id) => person(id, 'female')),
    person('alex', null),
    ...['ginette', 'carline', 'yves', 'yvette', 'colette', 'ana', 'bea'].map((id) => person(id, id === 'yves' ? 'male' : 'female')),
  ],
  relationships: [
    couple('jeanbaptiste', 'annemarie'),
    ...parents('joseph', 'jeanbaptiste', 'annemarie'),
    couple('joseph', 'marieclaire'),
    ...parents('jeanrobert', 'joseph', 'marieclaire'),
    ...parents('marietherese', 'joseph', 'marieclaire'),
    couple('marietherese', 'henri'),
    ...parents('lise', 'marietherese', 'henri'),
    couple('jeanrobert', 'nadege'),
    ...parents('maya', 'jeanrobert', 'nadege'),
    ...parents('fritz', 'jeanrobert', 'nadege'),
    ...parents('alex', 'jeanrobert', 'nadege'),
    couple('jeanrobert', 'guerline', { ended: true }),
    ...parents('sabine', 'jeanrobert', 'guerline'),
    ...parents('paul', 'guerline'),
    couple('fritz', 'fabienne'),
    ...parents('kervens', 'fritz', 'fabienne'),
    ...parents('tim', 'kervens'),
    couple('maya', 'stanley'),
    ...parents('naika', 'maya', 'stanley'),
    ...parents('mackenson', 'maya', 'stanley'),
    ...parents('naomie', 'naika'),
    couple('naika', 'ralph', { married: false }),
    ...parents('stanley', 'odette'),
    ...parents('wesner', 'odette'),
    couple('lucien', 'rose'),
    ...parents('nadege', 'lucien', 'rose'),
    ...parents('frantz', 'lucien', 'rose'),
    couple('frantz', 'mirlande'),
    ...parents('darline', 'frantz', 'mirlande'),
    couple('maya', 'pierre', { ended: true }),
    parent('maya', 'lucie', 'foster'),
    parent('ginette', 'odette'),
    couple('kervens', 'carline'),
    couple('darline', 'yves'),
    parent('yvette', 'pierre'),
    couple('joseph', 'colette'),
    parent('wesner', 'ana'),
    parent('wesner', 'bea'),
  ],
}

describe('relationsTo, as Maya', () => {
  const labels = relationsTo(tree, 'maya')

  it.each([
    { id: 'maya', want: 'You' },
    { id: 'jeanrobert', want: 'Your father' },
    { id: 'nadege', want: 'Your mother' },
    { id: 'joseph', want: 'Your grandfather' },
    { id: 'annemarie', want: 'Your great-grandmother' },
    { id: 'fritz', want: 'Your brother' },
    { id: 'alex', want: 'Your sibling' },
    { id: 'sabine', want: 'Your half-sister' },
    { id: 'naika', want: 'Your daughter' },
    { id: 'naomie', want: 'Your granddaughter' },
    { id: 'marietherese', want: 'Your aunt' },
    { id: 'henri', want: 'Your uncle' },
    { id: 'mirlande', want: 'Your aunt' },
    { id: 'lise', want: 'Your cousin' },
    { id: 'darline', want: 'Your cousin' },
    { id: 'kervens', want: 'Your nephew' },
    { id: 'tim', want: 'Your great-nephew' },
    { id: 'stanley', want: 'Your husband' },
    { id: 'pierre', want: 'Your former husband' },
    { id: 'odette', want: 'Your mother-in-law' },
    { id: 'wesner', want: 'Your brother-in-law' },
    { id: 'fabienne', want: 'Your sister-in-law' },
    { id: 'ralph', want: 'Your son-in-law' },
    { id: 'guerline', want: 'Your former stepmother' },
    { id: 'paul', want: 'Your former stepbrother' },
    { id: 'lucie', want: 'Your foster daughter' },
    { id: 'ginette', want: 'Your grandmother-in-law' },
    { id: 'carline', want: 'Your niece-in-law' },
    { id: 'yves', want: 'Your cousin-in-law' },
    { id: 'yvette', want: 'Your former mother-in-law' },
    { id: 'colette', want: 'Your step-grandmother' },
    { id: 'xavier', want: null },
  ])('$id: $want', ({ id, want }) => {
    expect(labels.get(id) ?? null).toBe(want)
  })
})

describe('relationsTo, from the other side', () => {
  it.each([
    { viewer: 'sabine', id: 'nadege', want: 'Your stepmother' },
    { viewer: 'paul', id: 'jeanrobert', want: 'Your former stepfather' },
    { viewer: 'paul', id: 'maya', want: 'Your former stepsister' },
    { viewer: 'ana', id: 'bea', want: 'Your sibling' },
    { viewer: 'stanley', id: 'naika', want: 'Your daughter' },
    { viewer: 'darline', id: 'maya', want: 'Your cousin' },
    { viewer: 'kervens', id: 'maya', want: 'Your aunt' },
    { viewer: 'lucie', id: 'maya', want: 'Your foster mother' },
    { viewer: 'lucie', id: 'naika', want: 'Your foster sister' },
    { viewer: 'naomie', id: 'jeanrobert', want: 'Your great-grandfather' },
    { viewer: 'stanley', id: 'jeanrobert', want: 'Your father-in-law' },
  ])('$viewer sees $id as $want', ({ viewer, id, want }) => {
    expect(relationsTo(tree, viewer).get(id) ?? null).toBe(want)
  })

  it('treats an adoptive parent as a parent, and ignores foster links for siblings', () => {
    const adopted: Tree = {
      persons: [person('a', 'female'), person('b', 'male'), person('c', 'female')],
      relationships: [parent('a', 'b', 'adoptive'), parent('a', 'c', 'foster')],
    }
    expect(relationsTo(adopted, 'b').get('a')).toBe('Your mother')
    expect(relationsTo(adopted, 'b').get('c')).toBe('Your foster sister')
  })
})
