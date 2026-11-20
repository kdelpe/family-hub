import { focusManager, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { type Contact, contactKeys } from '../../api/contacts'
import { type Person, personKeys } from '../../api/persons'
import type { Tree, TreePerson } from '../../api/tree'
import { PersonProfile } from './PersonProfile'

const slim = (id: string, first_name: string, gender: TreePerson['gender'], is_deceased = false): TreePerson => ({
  id,
  first_name,
  last_name: id === 'lucien' ? 'Monestime' : 'Pluviose',
  side: 'first',
  gender,
  is_deceased,
  bio_excerpt: null,
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
  cached = {},
}: {
  role?: 'member' | 'admin'
  contacts?: Contact[]
  answers?: Record<string, Answer | Answer[]>
  cached?: Record<string, { person: Person; contacts: Contact[] }>
} = {}) {
  const all: Record<string, Answer | Answer[]> = {
    'GET /api/persons/nadege': { status: 200, body: nadege },
    'GET /api/tree': { status: 200, body: tree },
    'GET /api/persons/nadege/contacts': { status: 200, body: { contacts } },
    ...answers,
  }
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const ids = url.startsWith('/api/persons?ids=') ? url.slice('/api/persons?ids='.length).split(',') : null
    if (ids && !answers[`GET ${url}`]) {
      const persons = everyone.filter((p) => ids.includes(p.id))
      return Promise.resolve(new Response(JSON.stringify({ persons }), { status: 200 }))
    }
    const queued = all[`${init?.method ?? 'GET'} ${url}`]
    // A list answers in turn, then keeps giving its last answer.
    const answer = Array.isArray(queued) ? (queued.length > 1 ? queued.shift() : queued[0]) : queued
    return answer
      ? Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
      : Promise.reject(new TypeError('Failed to fetch'))
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u', email: 'maya.pluviose@example.com', role, person_id: 'maya' })
  for (const [id, seen] of Object.entries(cached)) {
    client.setQueryData(personKeys.one(id), seen.person)
    client.setQueryData(contactKeys.of(id), seen.contacts)
  }
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
  focusManager.setFocused(undefined)
})

describe('PersonProfile', () => {
  it('asks only for the relatives it shows, never everyone', async () => {
    const fetch = renderPage()

    await screen.findByText('c. 1935 – 1998')

    const asked = fetch.mock.calls.map(([url]) => url).filter((url) => url.startsWith('/api/persons?'))
    expect(asked).toEqual([`/api/persons?ids=${['jeanrobert', 'lucien', 'maya', 'sabine'].join(',')}`])
  })

  it('shows who they are to you, their facts, and their family', async () => {
    renderPage()

    expect(await screen.findByRole('heading', { level: 1, name: 'Nadège Pluviose' })).toBeInTheDocument()
    expect(screen.getByText('Mother')).toBeInTheDocument()
    expect(screen.getByText('née Monestime · Birthday November 19 · Brooklyn, New York')).toBeInTheDocument()
    expect(screen.getByText('Taught third grade for thirty years.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Center the tree on Nadège' })).toHaveAttribute('href', '/tree?person=nadege')

    const family = screen.getByRole('region', { name: 'Nadège’s family' })
    await within(family).findByText('c. 1935 – 1998')
    const group = (title: string) => within(screen.getByRole('heading', { name: title }).parentElement!).getAllByRole('link')
    expect(group('Parents').map((a) => a.textContent)).toEqual(['LMLucien Monestimec. 1935 – 1998'])
    expect(group('Married to').map((a) => a.textContent)).toEqual(['JPJean-Robert PluvioseFather'])
    expect(group('Children').map((a) => a.textContent)).toEqual(['MPMaya PluvioseYou'])
    expect(group('Step-children').map((a) => a.textContent)).toEqual(['SPSabine PluvioseJean-Robert’s daughter'])
    expect(group('Children')[0]).toHaveAttribute('href', '/people/maya')
  })

  it('asks for a phone number, then says it was asked', async () => {
    const fetch = renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'phone', my_request_status: 'requested' } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    expect(await screen.findByText('Phone number requested', { selector: '[role=status]' })).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith(
      '/api/persons/nadege/contact-requests',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ kind: 'phone' }) }),
    )
  })

  it('starts fresh on the next profile after asking on this one', async () => {
    const phone: Contact[] = [{ kind: 'phone', requestable: true, my_request_status: 'none' }]
    renderPage({
      // Seen before, so the next profile's buttons render at once instead of after a load.
      cached: { jeanrobert: { person: full('jeanrobert'), contacts: phone } },
      answers: {
        'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'phone', my_request_status: 'requested' } },
        'GET /api/persons/jeanrobert': { status: 200, body: full('jeanrobert') },
        'GET /api/persons/jeanrobert/contacts': { status: 200, body: { contacts: phone } },
      },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))
    await screen.findByText('Phone number requested', { selector: '[role=status]' })
    fireEvent.click(screen.getByRole('link', { name: /Jean-Robert Pluviose/ }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Jean-Robert Pluviose' })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Request phone number' })).toBeInTheDocument()
    expect(screen.queryByText('Phone number requested')).not.toBeInTheDocument()
  })

  it('asks for contacts afresh on each visit, never showing what an earlier one saw', async () => {
    const sharedPhone: Contact = {
      id: 'c1',
      kind: 'phone',
      platform: null,
      label: null,
      value: '+1 718 555 0123',
      visibility: 'on_request',
      href: null,
      shared_with_you: true,
    }
    renderPage({
      answers: {
        'GET /api/persons/nadege/contacts': [
          { status: 200, body: { contacts: [sharedPhone] } },
          // Nadège took the share back while Maya was on Jean-Robert's profile.
          { status: 200, body: { contacts: [{ kind: 'phone', requestable: true, my_request_status: 'requested' }] } },
        ],
        'GET /api/persons/jeanrobert': { status: 200, body: full('jeanrobert') },
        'GET /api/persons/jeanrobert/contacts': { status: 200, body: { contacts: [] } },
      },
    })
    expect(await screen.findByText('Phone: +1 718 555 0123')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('link', { name: /Jean-Robert Pluviose/ }))
    await screen.findByRole('heading', { level: 1, name: 'Jean-Robert Pluviose' })

    fireEvent.click(await screen.findByRole('link', { name: /Nadège Pluviose/ }))

    expect(screen.getByRole('heading', { level: 1, name: 'Nadège Pluviose' })).toBeInTheDocument()
    expect(screen.queryByText('Phone: +1 718 555 0123')).not.toBeInTheDocument()
    expect(await screen.findByText('Phone number requested', { selector: ':not([role=status])' })).toBeInTheDocument()
    expect(screen.queryByText('Phone: +1 718 555 0123')).not.toBeInTheDocument()
  })

  it('drops the button when the number was already shared', async () => {
    renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'phone', my_request_status: 'shared' } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    await waitFor(() => expect(screen.queryByRole('button', { name: 'Request phone number' })).not.toBeInTheDocument())
    expect(screen.queryByText('Phone number requested')).not.toBeInTheDocument()
  })

  it.each([
    { name: 'the tree', key: 'GET /api/tree' },
    { name: 'the contacts', key: 'GET /api/persons/nadege/contacts' },
    { name: 'the relatives', key: `GET /api/persons?ids=${['jeanrobert', 'lucien', 'maya', 'sabine'].join(',')}` },
  ])('says when $name can’t load', async ({ key }) => {
    renderPage({ answers: { [key]: { status: 500, body: { error: { code: 'internal', message: 'x' } } } } })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })

  it('says why a login linked to nobody can’t ask', async () => {
    renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 403, body: { error: { code: 'forbidden', message: 'x' } } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Your login isn’t linked to you in the tree yet')
  })

  const contactsWith = (sharedWithYou: boolean): Contact[] => [
    {
      id: 'c1',
      kind: 'phone',
      platform: null,
      label: 'Mobile',
      value: '(555) 010-4121',
      visibility: 'on_request',
      href: null,
      shared_with_you: sharedWithYou,
    },
    {
      id: 'c2',
      kind: 'email',
      platform: null,
      label: null,
      value: 'nadege.p@example.com',
      visibility: 'family',
      href: null,
      shared_with_you: false,
    },
  ]

  // An admin gets every row, none marked shared with them.
  it.each([
    { name: 'shared with you', sharedWithYou: true, shown: true },
    { name: 'an admin’s view', sharedWithYou: false, shown: false },
  ])('shows a phone number only when $name', async ({ sharedWithYou, shown }) => {
    renderPage({ role: sharedWithYou ? 'member' : 'admin', contacts: contactsWith(sharedWithYou) })

    await screen.findByRole('heading', { level: 1, name: 'Nadège Pluviose' })
    await screen.findByRole('link', { name: 'Center the tree on Nadège' })
    expect(Boolean(screen.queryByText('Mobile: (555) 010-4121'))).toBe(shown)
    expect(screen.queryByText(/nadege\.p@example\.com/)).not.toBeInTheDocument()
  })

  it('hides a shared phone number once a later refresh fails', async () => {
    renderPage({
      answers: {
        'GET /api/persons/nadege/contacts': [
          { status: 200, body: { contacts: contactsWith(true) } },
          { status: 500, body: { error: { code: 'internal', message: 'x' } } },
        ],
      },
    })
    expect(await screen.findByText('Mobile: (555) 010-4121')).toBeInTheDocument()

    // Coming back to the window refreshes the contacts; this time the server fails.
    act(() => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.queryByText('Mobile: (555) 010-4121')).not.toBeInTheDocument()
  })

  it('hides a shared phone number while a refresh is on its way', async () => {
    const fetch = renderPage({ contacts: contactsWith(true) })
    expect(await screen.findByText('Mobile: (555) 010-4121')).toBeInTheDocument()
    const answer = fetch.getMockImplementation()!
    let respond: (value: Response) => void = () => {}
    fetch.mockImplementation((url, init) =>
      url === '/api/persons/nadege/contacts' ? new Promise((resolve) => (respond = resolve)) : answer(url, init),
    )

    act(() => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })

    await waitFor(() => expect(screen.queryByText('Mobile: (555) 010-4121')).not.toBeInTheDocument())
    act(() => respond(Response.json({ contacts: contactsWith(true) })))
    expect(await screen.findByText('Mobile: (555) 010-4121')).toBeInTheDocument()
  })

  it('says when there’s no such person', async () => {
    renderPage({ answers: { 'GET /api/persons/nadege': { status: 404, body: { error: { code: 'not_found', message: 'x' } } } } })

    expect(await screen.findByText(/We couldn’t find that person/)).toBeInTheDocument()
    expect(document.title).toBe('Not found — Pluviose-Monestime Family Hub')
    expect(screen.getByRole('link', { name: 'See everyone in People' })).toHaveAttribute('href', '/people')
  })
})
