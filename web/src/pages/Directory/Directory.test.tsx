import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import type { ContactMethod, DirectoryEntry } from '../../api/contacts'
import type { Tree } from '../../api/tree'
import { Directory } from './Directory'

const tree: Tree = {
  persons: [
    { id: 'maya', first_name: 'Maya', last_name: 'Pluviose', side: 'both', gender: 'female', is_deceased: false, bio_excerpt: null },
    { id: 'fritz', first_name: 'Fritz', last_name: 'Pluviose', side: 'both', gender: 'male', is_deceased: false, bio_excerpt: null },
    { id: 'nadege', first_name: 'Nadège', last_name: 'Pluviose', side: 'second', gender: 'female', is_deceased: false, bio_excerpt: null },
  ],
  relationships: [
    { id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'maya' },
    { id: 'r2', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'fritz' },
  ],
}

const method = (over: Partial<ContactMethod> & Pick<ContactMethod, 'id' | 'kind' | 'value'>): ContactMethod => ({
  platform: null,
  label: null,
  visibility: 'family',
  href: null,
  shared_with_you: false,
  ...over,
})

const people: DirectoryEntry[] = [
  {
    id: 'fritz',
    first_name: 'Fritz',
    last_name: 'Pluviose',
    side: 'both',
    contacts: [
      method({ id: 'c1', kind: 'email', value: 'fritz.p@example.com', visibility: 'family' }),
      method({ id: 'c2', kind: 'social', platform: 'youtube', value: '@fritzpluviose', href: 'https://www.youtube.com/@fritzpluviose' }),
      // The server makes no link of a value it can't vouch for.
      method({ id: 'c3', kind: 'social', platform: 'other', label: 'Mastodon', value: '@fritz@example.social' }),
      method({ id: 'c4', kind: 'phone', label: 'Mobile', value: '(555) 010-2231', visibility: 'on_request', shared_with_you: true }),
      // An admin gets rows like this; the directory still lists only what's shared with the family.
      method({ id: 'c5', kind: 'email', value: 'hidden@example.com', visibility: 'hidden' }),
    ],
  },
  { id: 'maya', first_name: 'Maya', last_name: 'Pluviose', side: 'both', contacts: [] },
  {
    id: 'nadege',
    first_name: 'Nadège',
    last_name: 'Pluviose',
    side: 'second',
    contacts: [
      { kind: 'address', requestable: true, my_request_status: 'none' },
      { kind: 'phone', requestable: true, my_request_status: 'requested' },
      { kind: 'whatsapp', requestable: true, my_request_status: 'none' },
    ],
  },
]

type Answer = { status: number; body?: unknown }

function renderPage(answers: Record<string, Answer> = {}) {
  const all: Record<string, Answer> = {
    'GET /api/directory': { status: 200, body: { people } },
    'GET /api/tree': { status: 200, body: tree },
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
  client.setQueryData(authKeys.me, { id: 'u', email: 'maya.pluviose@example.com', role: 'member', person_id: 'maya' })
  const router = createMemoryRouter([{ path: '/directory', element: <Directory /> }], { initialEntries: ['/directory'] })
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

const row = (name: string) => screen.getByRole('link', { name: new RegExp(name) }).closest('article')!

describe('Directory', () => {
  it('lists everyone but you, with only what they share with the family', async () => {
    renderPage()

    await screen.findByText('fritz.p@example.com')
    expect(screen.getAllByRole('article')).toHaveLength(2)
    expect(screen.queryByRole('link', { name: /Maya Pluviose/ })).not.toBeInTheDocument()

    const fritz = within(row('Fritz Pluviose'))
    expect(fritz.getByRole('link', { name: 'Email Fritz' })).toHaveAttribute('href', 'mailto:fritz.p@example.com')
    expect(fritz.getByRole('link', { name: 'Fritz on YouTube' })).toHaveAttribute('href', 'https://www.youtube.com/@fritzpluviose')
    expect(fritz.getByRole('link', { name: 'Fritz on YouTube' })).toHaveAttribute('rel', 'noopener noreferrer')
    expect(fritz.queryByRole('link', { name: /Mastodon/ })).not.toBeInTheDocument()
    expect(fritz.getByText('Mastodon: @fritz@example.social')).toBeInTheDocument()
    // Shared with you, but the number itself stays on the profile (design-notes §4.4).
    expect(fritz.queryByText(/010-2231/)).not.toBeInTheDocument()
    expect(fritz.getByRole('link', { name: 'Phone number shared with you' })).toHaveAttribute('href', '/people/fritz')
    expect(fritz.queryByText('hidden@example.com')).not.toBeInTheDocument()
    expect(row('Fritz Pluviose').querySelector('a')).toHaveAttribute('href', '/people/fritz')
    await fritz.findByText('Sibling')
  })

  it('says shared only where the server says so', async () => {
    const asAdmin = people.map((e) => ({
      ...e,
      contacts: e.contacts.map((c) => ('shared_with_you' in c ? { ...c, shared_with_you: false } : c)),
    }))
    renderPage({ 'GET /api/directory': { status: 200, body: { people: asAdmin } } })

    await screen.findByText('fritz.p@example.com')
    expect(screen.queryByRole('link', { name: /shared with you/ })).not.toBeInTheDocument()
  })

  it('tells an admin what’s on request, since they can’t ask', async () => {
    const asAdmin: DirectoryEntry[] = [
      {
        id: 'rose',
        first_name: 'Rose',
        last_name: 'Monestime',
        side: 'second',
        contacts: [
          method({ id: 'r1', kind: 'phone', value: '(555) 010-0001', visibility: 'on_request' }),
          method({ id: 'r2', kind: 'address', value: '1 Rue Sainte-Anne', visibility: 'on_request' }),
        ],
      },
    ]
    renderPage({ 'GET /api/directory': { status: 200, body: { people: asAdmin } } })

    expect(await screen.findByText('Phone number and address on request')).toBeInTheDocument()
    expect(screen.queryByText(/010-0001|Sainte-Anne/)).not.toBeInTheDocument()
  })

  it('offers what can be asked for, in order, and marks what was asked', async () => {
    renderPage()

    await screen.findByRole('button', { name: 'Request WhatsApp' })
    const nadege = within(row('Nadège Pluviose'))
    expect(nadege.getAllByRole('button').map((b) => b.textContent)).toEqual(['Request WhatsApp', 'Request address'])
    // Asked before, so shown but not announced.
    expect(nadege.getByText('Phone number requested', { selector: ':not([role=status])' })).toBeInTheDocument()
    expect(nadege.getAllByRole('status').map((r) => r.textContent)).toEqual(['', '', ''])
  })

  it('asks for an address and says it was asked', async () => {
    const fetch = renderPage({
      'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'address', my_request_status: 'requested' } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request address' }))

    expect(await screen.findByText('Address requested', { selector: '[role=status]' })).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith(
      '/api/persons/nadege/contact-requests',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ kind: 'address' }) }),
    )
  })

  it('searches by name', async () => {
    renderPage()

    await screen.findByText('fritz.p@example.com')
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search the directory' }), { target: { value: 'nadege' } })

    expect(screen.getAllByRole('article')).toHaveLength(1)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search the directory' }), { target: { value: 'zzz' } })
    expect(screen.getByText('Nobody here matches.')).toBeInTheDocument()
  })

  it('says when the directory can’t load', async () => {
    renderPage({ 'GET /api/directory': { status: 500, body: { error: { code: 'internal', message: 'x' } } } })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })
})
