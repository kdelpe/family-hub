import { describe, expect, it } from 'vitest'

import type { Tree, TreeRelationship } from '../api/tree'
import { familyLinks } from './familyLinks'
import { focusView } from './familyView'

let n = 0
const parent = (parent_id: string, child_id: string, subtype: 'biological' | 'adoptive' | 'foster' = 'biological'): TreeRelationship => ({
  id: `r${n++}`,
  type: 'parent_child',
  subtype,
  parent_id,
  child_id,
})
const couple = (a: string, b: string, ended = false): TreeRelationship => ({
  id: `r${n++}`,
  type: 'couple',
  subtype: 'married',
  ended,
  person_ids: [a, b],
})
const parents = (child: string, ...ps: string[]) => ps.map((p) => parent(p, child))

// Jean-Robert married Nadège (Maya, Fritz) after Guerline (Sabine); Maya and Stanley have Naïka, who has Naomie.
const tree: Tree = {
  persons: [],
  relationships: [
    ...parents('jeanrobert', 'joseph', 'marieclaire'),
    ...parents('nadege', 'lucien', 'rose'),
    ...parents('maya', 'jeanrobert', 'nadege'),
    ...parents('fritz', 'jeanrobert', 'nadege'),
    ...parents('sabine', 'jeanrobert', 'guerline'),
    ...parents('naika', 'maya', 'stanley'),
    parent('maya', 'tim', 'adoptive'),
    ...parents('naomie', 'naika'),
    couple('jeanrobert', 'nadege'),
    couple('jeanrobert', 'guerline', true),
    // Split up once and got back together.
    couple('maya', 'stanley', true),
    couple('maya', 'stanley'),
  ],
}
const family = familyLinks(tree)

describe('focusView', () => {
  it('shows the generations around Maya', () => {
    expect(focusView(family, 'maya')).toEqual({
      grandparents: [
        { of: 'jeanrobert', ids: ['joseph', 'marieclaire'] },
        { of: 'nadege', ids: ['lucien', 'rose'] },
      ],
      parents: ['jeanrobert', 'nadege'],
      siblings: [
        { id: 'fritz', half: false, foster: false },
        { id: 'sabine', half: true, foster: false },
      ],
      partners: ['stanley'],
      formerPartners: [],
      children: [
        { with: ['stanley'], ids: ['naika'] },
        { with: [], ids: ['tim'] },
      ],
      grandchildren: [{ of: 'naika', ids: ['naomie'] }],
    })
  })

  it('splits a parent’s children by the other parent, former partners apart', () => {
    const view = focusView(family, 'jeanrobert')

    expect(view.children).toEqual([
      { with: ['nadege'], ids: ['maya', 'fritz'] },
      { with: ['guerline'], ids: ['sabine'] },
    ])
    expect(view.partners).toEqual(['nadege'])
    expect(view.formerPartners).toEqual(['guerline'])
    expect(view.grandchildren).toEqual([{ of: 'maya', ids: ['naika', 'tim'] }])
  })

  it.each([
    { name: 'no parents recorded', focus: 'joseph', want: { parents: [], siblings: [], grandparents: [] } },
    { name: 'one parent recorded', focus: 'naomie', want: { parents: ['naika'], siblings: [], grandparents: [{ of: 'naika', ids: ['maya', 'stanley'] }] } },
  ])('$name', ({ focus, want }) => {
    expect(focusView(family, focus)).toMatchObject(want)
  })
})

describe('focusView loops', () => {
  it('shows each person once, where the family loops back on itself', () => {
    // Rose fosters her grandson Leo, and his parents Ana and Ben are half-siblings through Paul.
    const loops = familyLinks({
      persons: [],
      relationships: [
        ...parents('ana', 'paul', 'rose'),
        ...parents('ben', 'paul', 'mia'),
        ...parents('leo', 'ana', 'ben'),
        parent('rose', 'leo', 'foster'),
      ],
    })

    const view = focusView(loops, 'leo')
    expect(view.parents).toEqual(['ana', 'ben', 'rose'])
    expect(view.grandparents).toEqual([{ of: 'ana', ids: ['paul'] }, { of: 'ben', ids: ['mia'] }])
    expect(view.siblings).toEqual([])
  })

  it('keeps a child with a foster parent in their household', () => {
    const foster = familyLinks({
      persons: [],
      relationships: [...parents('kid1', 'maya', 'stanley'), ...parents('kid2', 'maya', 'stanley'), parent('lise', 'kid2', 'foster')],
    })

    expect(focusView(foster, 'maya').children).toEqual([{ with: ['stanley'], ids: ['kid1', 'kid2'] }])
  })

  it('puts the focus person’s own foster child apart from their birth parents, and marks foster siblings', () => {
    const fostering = familyLinks({
      persons: [],
      relationships: [
        ...parents('naika', 'maya', 'stanley'),
        ...parents('lise', 'ana', 'ben'),
        parent('maya', 'lise', 'foster'),
        ...parents('maya', 'nadege'),
        parent('nadege', 'odette', 'foster'),
      ],
    })

    expect(focusView(fostering, 'maya').children).toEqual([
      { with: ['stanley'], ids: ['naika'] },
      { with: [], ids: ['lise'] },
    ])
    expect(focusView(fostering, 'maya').siblings).toEqual([{ id: 'odette', half: false, foster: true }])
  })
})
