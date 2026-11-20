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
    ...['ginette', 'carline', 'yves', 'yvette', 'colette', 'ana', 'bea', 'dina', 'eva', 'flore', 'gina'].map((id) =>
      person(id, id === 'yves' ? 'male' : 'female'),
    ),
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
    parent('darline', 'dina'),
    parent('jeanbaptiste', 'eva'),
    parent('eva', 'flore'),
    parent('flore', 'gina'),
  ],
}

describe('relationsTo, as Maya', () => {
  const labels = relationsTo(tree, 'maya')

  it.each([
    { id: 'maya', want: 'You' },
    { id: 'jeanrobert', want: 'Father' },
    { id: 'nadege', want: 'Mother' },
    { id: 'joseph', want: 'Grandfather' },
    { id: 'annemarie', want: 'Great-grandmother' },
    { id: 'fritz', want: 'Brother' },
    { id: 'alex', want: 'Sibling' },
    { id: 'sabine', want: 'Half-sister' },
    { id: 'naika', want: 'Daughter' },
    { id: 'naomie', want: 'Granddaughter' },
    { id: 'marietherese', want: 'Aunt' },
    { id: 'henri', want: 'Uncle' },
    { id: 'mirlande', want: 'Aunt' },
    { id: 'lise', want: 'Cousin' },
    { id: 'darline', want: 'Cousin' },
    { id: 'kervens', want: 'Nephew' },
    { id: 'tim', want: 'Great-nephew' },
    { id: 'stanley', want: 'Husband' },
    { id: 'pierre', want: 'Former husband' },
    { id: 'odette', want: 'Mother-in-law' },
    { id: 'wesner', want: 'Brother-in-law' },
    { id: 'fabienne', want: 'Sister-in-law' },
    { id: 'ralph', want: 'Son-in-law' },
    { id: 'guerline', want: 'Former stepmother' },
    { id: 'paul', want: null },
    { id: 'lucie', want: 'Foster daughter' },
    { id: 'ginette', want: 'Grandmother-in-law' },
    { id: 'carline', want: 'Niece-in-law' },
    { id: 'yves', want: 'Cousin-in-law' },
    { id: 'yvette', want: 'Former mother-in-law' },
    { id: 'colette', want: 'Step-grandmother' },
    { id: 'dina', want: 'Cousin once removed' },
    { id: 'gina', want: 'Second cousin' },
    { id: 'flore', want: 'Cousin once removed' },
    { id: 'xavier', want: null },
  ])('$id: $want', ({ id, want }) => {
    expect(labels.get(id) ?? null).toBe(want)
  })
})

describe('relationsTo, from the other side', () => {
  it.each([
    { viewer: 'sabine', id: 'nadege', want: 'Stepmother' },
    { viewer: 'paul', id: 'jeanrobert', want: 'Former stepfather' },
    { viewer: 'paul', id: 'maya', want: null },
    { viewer: 'mirlande', id: 'maya', want: 'Niece' },
    { viewer: 'carline', id: 'maya', want: 'Aunt-in-law' },
    { viewer: 'ana', id: 'bea', want: 'Sibling' },
    { viewer: 'stanley', id: 'naika', want: 'Daughter' },
    { viewer: 'darline', id: 'maya', want: 'Cousin' },
    { viewer: 'kervens', id: 'maya', want: 'Aunt' },
    { viewer: 'lucie', id: 'maya', want: 'Foster mother' },
    { viewer: 'lucie', id: 'naika', want: 'Foster sister' },
    { viewer: 'naomie', id: 'jeanrobert', want: 'Great-grandfather' },
    { viewer: 'stanley', id: 'jeanrobert', want: 'Father-in-law' },
  ])('$viewer sees $id as $want', ({ viewer, id, want }) => {
    expect(relationsTo(tree, viewer).get(id) ?? null).toBe(want)
  })

  it.each([
    { name: 'a great-aunt or great-uncle', viewer: 'v', id: 'x', want: 'Great-aunt or great-uncle' },
    { name: 'an in-law', viewer: 'p', id: 'y', want: 'Aunt-in-law or uncle-in-law' },
  ])('words $name of unknown gender in full', ({ viewer, id, want }) => {
    const unknown: Tree = {
      persons: ['v', 'm', 'g', 'gg', 'x', 'p', 'y'].map((i) => person(i, i === 'x' || i === 'y' ? null : 'female')),
      relationships: [
        parent('m', 'v'),
        parent('g', 'm'),
        parent('gg', 'g'),
        parent('gg', 'x'),
        couple('v', 'p', { married: false }),
        parent('g', 'y'),
      ],
    }
    expect(relationsTo(unknown, viewer).get(id)).toBe(want)
  })

  it('names far cousins with real ordinals', () => {
    const line = (prefix: string, n: number) => Array.from({ length: n }, (_, i) => `${prefix}${i}`)
    const left = line('l', 23)
    const right = line('r', 23)
    const far: Tree = {
      persons: ['root', ...left, ...right].map((i) => person(i, 'female')),
      relationships: [
        parent('root', 'l0'),
        parent('root', 'r0'),
        ...left.slice(1).map((id, i) => parent(left[i]!, id)),
        ...right.slice(1).map((id, i) => parent(right[i]!, id)),
      ],
    }
    expect(relationsTo(far, 'l21').get('r21')).toBe('21st cousin')
    expect(relationsTo(far, 'l10').get('r10')).toBe('10th cousin')
    expect(relationsTo(far, 'l1').get('r4')).toBe('Cousin 3 times removed')
  })

  it('puts -in-law before the removal, and names a foster sibling over a distant cousin', () => {
    const t: Tree = {
      persons: ['me', 'mom', 'gm', 'aunt', 'cousin', 'kid', 'kidwife', 'home', 'fos'].map((i) => person(i, 'female')),
      relationships: [
        parent('mom', 'me'),
        parent('gm', 'mom'),
        parent('gm', 'aunt'),
        parent('aunt', 'cousin'),
        parent('cousin', 'kid'),
        couple('kid', 'kidwife'),
        parent('mom', 'fos', 'foster'),
        parent('aunt', 'home'),
        parent('home', 'fos'),
      ],
    }
    const labels = relationsTo(t, 'me')
    expect(labels.get('kid')).toBe('Cousin once removed')
    expect(labels.get('kidwife')).toBe('Cousin-in-law once removed')
    expect(labels.get('fos')).toBe('Foster sister')
  })

  it('prefers a current tie to an ended one, across both in-law sides', () => {
    const mixed: Tree = {
      persons: ['me', 'bro', 'ex', 'wife', 'exparent'].map((i) => person(i, i === 'wife' || i === 'me' ? 'female' : 'male')),
      relationships: [
        parent('mom', 'me'),
        parent('mom', 'bro'),
        couple('me', 'ex', { ended: true }),
        parent('exparent', 'ex'),
        parent('exparent', 'wife'),
        couple('bro', 'wife'),
      ],
    }
    expect(relationsTo(mixed, 'me').get('wife')).toBe('Sister-in-law')
  })

  it('treats an adoptive parent as a parent, and ignores foster links for siblings', () => {
    const adopted: Tree = {
      persons: [person('a', 'female'), person('b', 'male'), person('c', 'female')],
      relationships: [parent('a', 'b', 'adoptive'), parent('a', 'c', 'foster')],
    }
    expect(relationsTo(adopted, 'b').get('a')).toBe('Mother')
    expect(relationsTo(adopted, 'b').get('c')).toBe('Foster sister')
  })
})
