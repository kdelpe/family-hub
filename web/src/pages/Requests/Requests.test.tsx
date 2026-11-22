import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import type { ContactRequest } from '../../api/contacts'
import { personKeys } from '../../api/persons'
import { Requests } from './Requests'

const request = (id: string, name: string, status: ContactRequest['status'], kind: ContactRequest['kind'] = 'phone'): ContactRequest => ({
  id,
  kind,
  status,
  created_at: '2026-11-01T10:00:00Z',
  decided_at: status === 'pending' ? null : '2026-11-02T10:00:00Z',
  from: { person_id: id, name },
})

const all = [
  request('r1', 'Stanley Jean-Louis', 'pending'),
  request('r2', 'Nadège Pluviose', 'pending', 'address'),
  request('r3', 'Fritz Pluviose', 'shared', 'whatsapp'),
  request('r4', 'Guerline Désir', 'declined', 'address'),
  request('r5', 'Esther Pluviose', 'revoked'),
]

// Answers the tree, the list (or fails it once listFails is set) and every decision, which it remembers.
function renderPage(requests: ContactRequest[], personId: string | null = 'maya', server = { listFails: false }) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        const [, id, decision] = /contact-requests\/([^/]+)\/(\w+)$/.exec(url) ?? []
        const status = ({ share: 'shared', decline: 'declined', revoke: 'revoked' } as const)[decision as 'share']
        requests = requests.map((r) => (r.id === id ? { ...r, status } : r))
        return Promise.resolve(new Response(null, { status: 204 }))
      }
      if (url === '/api/tree') {
        return Promise.resolve(Response.json({ persons: [], relationships: [] }))
      }
      return server.listFails ? Promise.reject(new TypeError('Failed to fetch')) : Promise.resolve(Response.json({ person_id: 'maya', requests }))
    }),
  )
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  lastClient = client
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'member', person_id: personId })
  client.setQueryData(personKeys.one('maya'), { id: 'maya', first_name: 'Maya', last_name: 'Pluviose' })
  render(
    <QueryClientProvider client={client}>
      <Requests />
    </QueryClientProvider>,
  )
}

const panel = () => within(screen.getByRole('tabpanel'))
let lastClient: QueryClient
const client = () => lastClient

afterEach(() => vi.unstubAllGlobals())

describe('Requests', () => {
  it('opens on the waiting requests, counted on their tab', async () => {
    renderPage(all)

    expect(await panel().findByText('Stanley Jean-Louis')).toBeInTheDocument()
    expect(panel().getByText('Nadège Pluviose')).toBeInTheDocument()
    expect(panel().getAllByRole('button', { name: 'Share' })).toHaveLength(2)
    // No relation in an empty tree: just when they asked.
    expect(panel().getAllByText(/^Asked /)).toHaveLength(2)
    expect(screen.getByRole('tab', { name: 'Waiting, 2 requests' })).toHaveAttribute('aria-selected', 'true')
  })

  it.each([
    { tab: 'Answered', shows: ['✓ Shared', 'Not shared', 'Stopped sharing'], hides: 'Stanley Jean-Louis' },
    { tab: 'Shared with', shows: ['Fritz Pluviose · WhatsApp number'], hides: 'Guerline Désir' },
  ])('lists $tab on its tab', async ({ tab, shows, hides }) => {
    renderPage(all)
    await panel().findByText('Stanley Jean-Louis')

    fireEvent.click(screen.getByRole('tab', { name: tab }))

    for (const text of shows) {
      expect(panel().getByText(text)).toBeInTheDocument()
    }
    expect(panel().queryByText(hides)).toBeNull()
  })

  it.each([
    { tab: 'Waiting', empty: 'No requests right now.' },
    { tab: 'Answered', empty: 'Nothing answered yet.' },
    { tab: 'Shared with', empty: 'You haven’t shared your details with anyone.' },
  ])('says so when $tab is empty', async ({ tab, empty }) => {
    renderPage([])
    await panel().findByText('No requests right now.')

    fireEvent.click(screen.getByRole('tab', { name: tab }))

    expect(panel().getByText(empty)).toBeInTheDocument()
  })

  it('lists answers newest answered first, not newest asked', async () => {
    const asked = (id: string, name: string, created: string, decided: string) => ({ ...request(id, name, 'declined'), created_at: created, decided_at: decided })
    renderPage([asked('old', 'Asked Long Ago', '2026-10-01T10:00:00Z', '2026-11-20T10:00:00Z'), asked('new', 'Asked Lately', '2026-11-10T10:00:00Z', '2026-11-11T10:00:00Z')])
    await panel().findByText('No requests right now.')

    fireEvent.click(screen.getByRole('tab', { name: 'Answered' }))

    expect(panel().getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['Asked Long Ago', 'Asked Lately'])
  })

  it('takes back a share given on this visit, saying so where focus lands', async () => {
    renderPage([request('r1', 'Stanley Jean-Louis', 'pending')])
    fireEvent.click(await panel().findByRole('button', { name: 'Share' }))
    fireEvent.click(panel().getByRole('button', { name: 'Yes, share' }))
    await panel().findByText('Shared with Stanley.')

    fireEvent.click(screen.getByRole('tab', { name: 'Shared with' }))
    fireEvent.click(panel().getByRole('button', { name: 'Stop sharing' }))
    fireEvent.click(panel().getByRole('button', { name: 'Yes, stop' }))

    expect(await panel().findByText('Stopped sharing your phone number with Stanley.')).toHaveFocus()
    fireEvent.click(screen.getByRole('tab', { name: 'Waiting' }))
    expect(panel().queryByText('Shared with Stanley.')).toBeNull()
  })

  it('keeps the list when a reread fails, saying so', async () => {
    const server = { listFails: false }
    renderPage([request('r1', 'Stanley Jean-Louis', 'pending')], 'maya', server)
    await panel().findByText('Stanley Jean-Louis')
    server.listFails = true

    await act(() => client().refetchQueries())

    expect(await panel().findByRole('alert')).toBeInTheDocument()
    expect(panel().getByText('Stanley Jean-Louis')).toBeInTheDocument()
  })

  it('tells a login linked to nobody that nobody can ask yet', () => {
    renderPage([], null)

    expect(screen.getByText(/isn’t linked to you in the tree yet/)).toBeInTheDocument()
    expect(screen.queryByRole('tablist')).toBeNull()
  })
})
