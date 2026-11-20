import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import type { Tree, TreePerson } from '../../api/tree'
import { People } from './People'

const person = (id: string, first_name: string, over: Partial<TreePerson> = {}): TreePerson => ({
  id,
  first_name,
  last_name: 'Pluviose',
  side: 'first',
  gender: 'female',
  is_deceased: false,
  ...over,
})

const tree: Tree = {
  persons: [
    person('maya', 'Maya', { side: 'both' }),
    person('nadege', 'Nadège', { side: 'second' }),
    person('anne', 'Anne-Marie', { is_deceased: true }),
    person('stanley', 'Stanley', { last_name: 'Jean-Louis', side: 'married_in', gender: 'male' }),
  ],
  relationships: [
    { id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'maya' },
    { id: 'r2', type: 'couple', subtype: 'married', ended: false, person_ids: ['maya', 'stanley'] },
  ],
}

function renderPage(personId: string | null = 'maya') {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      const body =
        url === '/api/tree'
          ? tree
          : url === '/api/family/summary'
            ? { people: 4, generations: 2, earliest_record: { year: 1900, circa: true, place: 'Jacmel' } }
            : undefined
      return body
        ? Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
        : Promise.reject(new TypeError('Failed to fetch'))
    }),
  )
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u', email: 'maya.pluviose@example.com', role: 'member', person_id: personId })
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <People />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const cards = () => within(screen.getByRole('list')).getAllByRole('link')
const names = () => cards().map((c) => within(c).getByText(/^\S+ (Pluviose|Jean-Louis)$/).textContent)

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('People', () => {
  it('shows everyone A–Z, each with how they’re related to you', async () => {
    renderPage()

    expect(await screen.findByText('Earliest record: c. 1900, Jacmel')).toBeInTheDocument()
    expect(names()).toEqual(['Anne-Marie Pluviose', 'Maya Pluviose', 'Nadège Pluviose', 'Stanley Jean-Louis'])
    const [anne, maya, nadege, stanley] = cards()
    expect(anne).toHaveTextContent('In memory')
    expect(maya).toHaveTextContent('You')
    expect(maya).toHaveTextContent('Both sides')
    expect(nadege).toHaveTextContent('Your mother')
    expect(stanley).toHaveTextContent('Your husband')
    expect(stanley).toHaveTextContent('Married in')
    expect(nadege).toHaveAttribute('href', '/people/nadege')
  })

  it.each([
    { filter: 'Living', want: ['Maya Pluviose', 'Nadège Pluviose', 'Stanley Jean-Louis'] },
    { filter: 'Remembered', want: ['Anne-Marie Pluviose'] },
    { filter: 'Pluviose side', want: ['Anne-Marie Pluviose', 'Maya Pluviose'] },
    { filter: 'Monestime side', want: ['Maya Pluviose', 'Nadège Pluviose'] },
  ])('filters to $filter', async ({ filter, want }) => {
    renderPage()
    await screen.findByText('Stanley Jean-Louis')

    fireEvent.click(screen.getByRole('button', { name: filter }))

    expect(screen.getByRole('button', { name: filter })).toHaveAttribute('aria-pressed', 'true')
    expect(names()).toEqual(want)
  })

  it('searches by name, accents or not', async () => {
    renderPage()
    await screen.findByText('Stanley Jean-Louis')

    fireEvent.change(screen.getByLabelText('Search people'), { target: { value: 'nadege' } })
    expect(names()).toEqual(['Nadège Pluviose'])

    fireEvent.change(screen.getByLabelText('Search people'), { target: { value: 'zz' } })
    expect(screen.getByText('Nobody here matches.')).toBeInTheDocument()
  })

  it('leaves out relations for a login linked to nobody', async () => {
    renderPage(null)

    await screen.findByText('Stanley Jean-Louis')
    expect(screen.queryByText(/^You$|^Your /)).not.toBeInTheDocument()
  })
})
