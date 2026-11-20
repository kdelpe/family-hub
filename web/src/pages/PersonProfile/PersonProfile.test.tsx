import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import type { Contact } from '../../api/contacts'
import type { Person } from '../../api/persons'
import type { Tree, TreePerson } from '../../api/tree'
import { PersonProfile } from './PersonProfile'

const slim = (id: string, first_name: string, gender: TreePerson['gender'], is_deceased = false): TreePerson => ({
  id,
  first_name,
  last_name: id === 'lucien' ? 'Monestime' : 'Pluviose',
  side: 'first',
  gender,
  is_deceased,
})

const tree: Tree = {
  persons: [
    slim('lucien', 'Lucien', 'male', true),
    slim('nadege', 'Nadège', 'female'),
    slim('jeanrobert', 'Jean-Robert', 'male'),
    slim('maya', 'Maya', 'female'),
    slim('sabine', 'Sabine', 'female'),
  ],
  relationships: [
    { id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'lucien', child_id: 'nadege' },
    { id: 'r2', type: 'couple', subtype: 'married', ended: false, person_ids: ['jeanrobert', 'nadege'] },
    { id: 'r3', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'maya' },
    { id: 'r4', type: 'parent_child', subtype: 'biological', parent_id: 'jeanrobert', child_id: 'maya' },
    { id: 'r5', type: 'parent_child', subtype: 'biological', parent_id: 'jeanrobert', child_id: 'sabine' },
  ],
}

const full = (id: string, over: Partial<Person> = {}): Person => ({
  ...slim(id, tree.persons.find((p) => p.id === id)!.first_name, null),
  maiden_name: null,
  birth_place: null,
  birth_date: null,
  birth_date_precision: null,
  death_date: null,
  death_date_precision: null,
  is_deceased: false,
  bio: null,
  ...over,
})

const nadege = full('nadege', {
  maiden_name: 'Monestime',
  birth_date: '--11-19',
  birth_date_precision: 'day',
  birth_place: 'Brooklyn, New York',
  bio: 'Taught third grade for thirty years.',
})
const everyone = [
  full('lucien', { is_deceased: true, birth_date: '1935', birth_date_precision: 'circa', death_date: '1998', death_date_precision: 'year' }),
  nadege,
  full('jeanrobert'),
  full('maya'),
  full('sabine'),
]

type Answer = { status: number; body?: unknown }

function renderPage({
  role = 'member',
  contacts = [{ kind: 'phone', requestable: true, my_request_status: 'none' }],
  answers = {},
}: { role?: 'member' | 'admin'; contacts?: Contact[]; answers?: Record<string, Answer> } = {}) {
  const all: Record<string, Answer> = {
    'GET /api/persons/nadege': { status: 200, body: nadege },
    'GET /api/tree': { status: 200, body: tree },
    'GET /api/persons': { status: 200, body: { persons: everyone } },
    'GET /api/persons/nadege/contacts': { status: 200, body: { contacts } },
    ...answers,
  }
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const answer = all[`${init?.method ?? 'GET'} ${url}`]
    return answer
      ? Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
      : Promise.reject(new TypeError('Failed to fetch'))
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u', email: 'maya.pluviose@example.com', role, person_id: 'maya' })
  const router = createMemoryRouter([{ path: '/people/:id', element: <PersonProfile /> }], {
    initialEntries: ['/people/nadege'],
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return fetch
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('PersonProfile', () => {
  it('shows who they are to you, their facts, and their family', async () => {
    renderPage()

    expect(await screen.findByRole('heading', { level: 1, name: 'Nadège Pluviose' })).toBeInTheDocument()
    expect(screen.getByText('Your mother')).toBeInTheDocument()
    expect(screen.getByText('née Monestime · Birthday November 19 · Brooklyn, New York')).toBeInTheDocument()
    expect(screen.getByText('Taught third grade for thirty years.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Center the tree on Nadège' })).toHaveAttribute('href', '/tree?person=nadege')

    const family = screen.getByRole('region', { name: 'Nadège’s family' })
    await within(family).findByText('c. 1935 – 1998')
    const group = (title: string) => within(screen.getByRole('heading', { name: title }).parentElement!).getAllByRole('link')
    expect(group('Parents').map((a) => a.textContent)).toEqual(['LMLucien Monestimec. 1935 – 1998'])
    expect(group('Married to').map((a) => a.textContent)).toEqual(['JPJean-Robert PluvioseYour father'])
    expect(group('Children').map((a) => a.textContent)).toEqual(['MPMaya PluvioseYou'])
    expect(group('Step-children').map((a) => a.textContent)).toEqual(['SPSabine PluvioseJean-Robert’s daughter'])
    expect(group('Children')[0]).toHaveAttribute('href', '/people/maya')
  })

  it('asks for a phone number, then says it was asked', async () => {
    const fetch = renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'phone', my_request_status: 'requested' } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    expect(await screen.findByText('✓ Phone number requested')).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith(
      '/api/persons/nadege/contact-requests',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ kind: 'phone' }) }),
    )
  })

  it('says why a login linked to nobody can’t ask', async () => {
    renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 403, body: { error: { code: 'forbidden', message: 'x' } } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Your login isn’t linked to you in the tree yet')
  })

  const shared: Contact[] = [
    { id: 'c1', kind: 'phone', platform: null, label: 'Mobile', value: '(555) 010-4121', visibility: 'on_request' },
    { id: 'c2', kind: 'email', platform: null, label: null, value: 'nadege.p@example.com', visibility: 'family' },
  ]

  it.each([
    { role: 'member' as const, shown: true },
    { role: 'admin' as const, shown: false },
  ])('shows a shared phone number to a $role: $shown', async ({ role, shown }) => {
    renderPage({ role, contacts: shared })

    await screen.findByRole('heading', { level: 1, name: 'Nadège Pluviose' })
    await screen.findByRole('link', { name: 'Center the tree on Nadège' })
    expect(Boolean(screen.queryByText('Mobile: (555) 010-4121'))).toBe(shown)
    expect(screen.queryByText(/nadege\.p@example\.com/)).not.toBeInTheDocument()
  })

  it('says when there’s no such person', async () => {
    renderPage({ answers: { 'GET /api/persons/nadege': { status: 404, body: { error: { code: 'not_found', message: 'x' } } } } })

    expect(await screen.findByText(/We couldn’t find that person/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See everyone in People' })).toHaveAttribute('href', '/people')
  })
})
