import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { describe, expect, it } from 'vitest'

import { authKeys } from '../../api/auth'
import { type Home, homeKeys } from '../../api/home'
import { personKeys } from '../../api/persons'
import { treeKeys } from '../../api/tree'
import { HomeHearth } from './HomeHearth'

const fridayEvening = new Date('2026-11-20T18:30')

const nothing: Home = { birthdays: [], newcomers: [] }

const person = (id: string, first_name: string, last_name = 'Pluviose') => ({ id, first_name, last_name, side: 'first', gender: 'female', is_deceased: false })

// Maya (p1) with her mother Nadège and her cousin Naomie.
const tree = {
  persons: [person('p1', 'Maya'), person('nadege', 'Nadège'), person('gma', 'Rose'), person('aunt', 'Naïka'), person('naomie', 'Naomie', 'Désir'), person('far', 'Distant', 'Stranger')],
  relationships: [
    { id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'p1' },
    { id: 'r2', type: 'parent_child', subtype: 'biological', parent_id: 'gma', child_id: 'nadege' },
    { id: 'r3', type: 'parent_child', subtype: 'biological', parent_id: 'gma', child_id: 'aunt' },
    { id: 'r4', type: 'parent_child', subtype: 'biological', parent_id: 'aunt', child_id: 'naomie' },
  ],
}

function renderHome(personId: string | null, home: Home = nothing) {
  const client = new QueryClient()
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: personId })
  client.setQueryData(personKeys.one('p1'), { id: 'p1', first_name: 'Maya', last_name: 'Pluviose' })
  client.setQueryData(homeKeys.all, home)
  client.setQueryData(treeKeys.all, tree)
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={createMemoryRouter([{ path: '/', element: <HomeHearth now={fridayEvening} /> }])} />
    </QueryClientProvider>,
  )
}

describe('HomeHearth', () => {
  it('greets the viewer by name for the time of day', () => {
    renderHome('p1')

    expect(screen.getByText('Friday evening')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'Good evening, Maya' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open the family tree' })).toHaveAttribute('href', '/tree')
    expect(screen.getByRole('link', { name: 'See new photos' })).toHaveAttribute('href', '/events')
  })

  it('greets a login linked to nobody without a name', () => {
    renderHome(null)

    expect(screen.getByRole('heading', { level: 1, name: 'Good evening' })).toBeInTheDocument()
  })

  it('lists the birthdays coming up, soonest first, and says when there are none', () => {
    renderHome('p1', {
      newcomers: [],
      birthdays: [
        { id: 'nadege', first_name: 'Nadège', last_name: 'Pluviose', on: '2026-11-20', in_days: 0 },
        { id: 'mack', first_name: 'Mackenson', last_name: 'Pluviose', on: '2026-11-24', in_days: 4 },
      ],
    })

    const card = within(screen.getByRole('region', { name: 'Coming up' }))
    expect(card.getByRole('link', { name: 'Nadège’s birthday' })).toHaveAttribute('href', '/people/nadege')
    expect(card.getByText('Today')).toBeInTheDocument()
    expect(card.getByText('In 4 days')).toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'New in the family' })).getByText('Nobody new in the last month.')).toBeInTheDocument()
  })

  it('shows three birthdays, then the rest on See all', () => {
    const names = ['Ana', 'Bruno', 'Carla', 'Didier', 'Elise']
    renderHome('p1', {
      newcomers: [],
      birthdays: names.map((first_name, i) => ({ id: first_name, first_name, last_name: null, on: `2026-11-2${i}`, in_days: i })),
    })
    const card = within(screen.getByRole('region', { name: 'Coming up' }))
    expect(card.getAllByRole('link').map((a) => a.textContent)).toEqual(['Ana’s birthday', 'Bruno’s birthday', 'Carla’s birthday'])

    fireEvent.click(card.getByRole('button', { name: 'See all' }), { detail: 0 })

    expect(card.getAllByRole('link')).toHaveLength(5)
    expect(card.getByRole('link', { name: 'Didier’s birthday' })).toHaveFocus()
    expect(card.queryByRole('button', { name: 'See all' })).not.toBeInTheDocument()
  })

  it.each([
    { name: 'today', id: 'mack', in_days: 0, line: 'Mackenson’s birthday is today.' },
    { name: 'tomorrow', id: 'mack', in_days: 1, line: 'Mackenson’s birthday is tomorrow.' },
    { name: 'in a few days', id: 'mack', in_days: 3, line: 'Mackenson’s birthday is in 3 days.' },
    { name: 'the viewer’s own', id: 'p1', in_days: 3, line: 'Your birthday is in 3 days.' },
  ])('says in the hero when the next birthday is: $name', ({ id, in_days, line }) => {
    renderHome('p1', {
      newcomers: [],
      birthdays: [
        { id, first_name: 'Mackenson', last_name: 'Pluviose', on: '2026-11-23', in_days },
        { id: 'later', first_name: 'Later', last_name: null, on: '2026-11-30', in_days: 10 },
      ],
    })

    expect(within(screen.getByRole('region', { name: 'Welcome' })).getByText(line)).toBeInTheDocument()
  })

  it('says nothing about birthdays in the hero when none is coming up', () => {
    renderHome('p1')

    expect(within(screen.getByRole('region', { name: 'Welcome' })).queryByText(/birthday/)).toBeNull()
  })

  it('says so when no birthday is coming up', () => {
    renderHome('p1')

    expect(within(screen.getByRole('region', { name: 'Coming up' })).getByText('No birthdays in the next two weeks.')).toBeInTheDocument()
  })

  it('names newcomers by their relation to you and when they were added', async () => {
    renderHome('p1', {
      birthdays: [],
      newcomers: [
        { id: 'naomie', first_name: 'Naomie', last_name: 'Désir', added_at: new Date('2026-11-17T12:00').toISOString() },
        { id: 'far', first_name: 'Distant', last_name: 'Stranger', added_at: new Date('2026-11-20T09:00').toISOString() },
      ],
    })

    const card = within(screen.getByRole('region', { name: 'New in the family' }))
    expect(card.getByRole('link', { name: 'Naomie Désir' })).toHaveAttribute('href', '/people/naomie')
    expect(await card.findByText('Cousin · added Tuesday')).toBeInTheDocument()
    expect(card.getByText('Added today')).toBeInTheDocument()
  })

  it('shows a card for a name pointed at, with links to their profile and their place in the tree', async () => {
    renderHome('p1', {
      birthdays: [],
      newcomers: [{ id: 'naomie', first_name: 'Naomie', last_name: 'Désir', added_at: new Date('2026-11-17T12:00').toISOString() }],
    })

    fireEvent.pointerEnter(await screen.findByRole('link', { name: 'Naomie Désir' }), { pointerType: 'mouse' })

    const inTree = await screen.findByRole('link', { name: 'In the tree' })
    const card = within(inTree.parentElement!.parentElement!)
    expect(card.getByText('Cousin')).toBeInTheDocument()
    expect(card.getByRole('link', { name: 'Profile' })).toHaveAttribute('href', '/people/naomie')
    expect(inTree).toHaveAttribute('href', '/tree?person=naomie')
  })

  it('doesn’t open a name’s card on keyboard focus', async () => {
    renderHome('p1', {
      birthdays: [],
      newcomers: [{ id: 'naomie', first_name: 'Naomie', last_name: 'Désir', added_at: new Date('2026-11-17T12:00').toISOString() }],
    })
    const name = await screen.findByRole('link', { name: 'Naomie Désir' })

    act(() => name.focus())
    await new Promise((r) => setTimeout(r, 400))

    expect(name).toHaveFocus()
    expect(screen.queryByRole('link', { name: 'In the tree' })).toBeNull()
  })
})
