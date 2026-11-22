import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
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

function renderPage(requests: ContactRequest[], personId: string | null = 'maya') {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(
        Response.json(url === '/api/tree' ? { persons: [], relationships: [] } : { person_id: 'maya', requests }),
      ),
    ),
  )
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'member', person_id: personId })
  client.setQueryData(personKeys.one('maya'), { id: 'maya', first_name: 'Maya', last_name: 'Pluviose' })
  render(
    <QueryClientProvider client={client}>
      <Requests />
    </QueryClientProvider>,
  )
}

const panel = () => within(screen.getByRole('tabpanel'))

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

  it('tells a login linked to nobody that nobody can ask yet', () => {
    renderPage([], null)

    expect(screen.getByText(/isn’t linked to you in the tree yet/)).toBeInTheDocument()
    expect(screen.queryByRole('tablist')).toBeNull()
  })
})
