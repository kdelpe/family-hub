import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { ContactRequest } from '../../api/contacts'
import { RequestsCard } from './RequestsCard'

const request = (over: Partial<ContactRequest> & Pick<ContactRequest, 'id'>): ContactRequest => ({
  kind: 'phone',
  status: 'pending',
  created_at: '2026-11-01T10:00:00Z',
  decided_at: null,
  from: { person_id: 'darline', name: 'Darline Monestime' },
  ...over,
})

const darline = request({ id: 'r1' })
const peterson = request({
  id: 'r2',
  kind: 'address',
  created_at: '2026-11-02T10:00:00Z',
  from: { person_id: 'peterson', name: 'Peterson Pluviose' },
})
const naika = request({
  id: 'r3',
  kind: 'whatsapp',
  status: 'shared',
  decided_at: '2026-11-03T10:00:00Z',
  from: { person_id: 'naika', name: 'Naïka Jean-Louis' },
})

type Answer = { status: number; body?: unknown }

const listed = (requests: ContactRequest[]): Answer => ({ status: 200, body: { person_id: 'maya', requests } })

function renderCard(answers: Record<string, Answer>) {
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    // Read at call time, so a test can change what the server lists midway.
    const answer = answers[`${init?.method ?? 'GET'} ${url}`]
    return answer
      ? Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
      : Promise.reject(new TypeError('Failed to fetch'))
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={client}>
      <RequestsCard personId="maya" />
    </QueryClientProvider>,
  )
  return { fetch, client }
}

const posted = (fetch: ReturnType<typeof renderCard>['fetch'], url: string) =>
  fetch.mock.calls.some(([u, init]) => u === url && init?.method === 'POST')

// Until the list is read again and the card has caught up.
async function settled(client: QueryClient) {
  await vi.waitFor(() => expect(client.isFetching()).toBe(0))
  await act(() => new Promise((resolve) => setTimeout(resolve, 0)))
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('RequestsCard', () => {
  it('lists who asked, oldest first, then who you’ve shared with', async () => {
    renderCard({ 'GET /api/me/contact-requests': listed([peterson, naika, darline]) })

    await screen.findAllByRole('button', { name: 'Share' })
    expect(screen.getAllByText(/asked for your/).map((e) => e.textContent)).toEqual([
      'Darline Monestime asked for your phone number.',
      'Peterson Pluviose asked for your address.',
    ])
    expect(screen.getAllByRole('button', { name: 'Don’t share' })).toHaveLength(2)
    expect(screen.getByRole('heading', { name: 'Shared with' })).toBeInTheDocument()
    expect(screen.getByRole('listitem')).toHaveTextContent('Naïka Jean-Louis · WhatsApp number')
    expect(
      screen.getByText(
        'Sharing shows that one person every detail of that kind you’ve set to “Family can ask”, including ones you add later. You can stop sharing any time.',
      ),
    ).toBeInTheDocument()
  })

  it('says when no one has asked', async () => {
    renderCard({ 'GET /api/me/contact-requests': listed([]) })

    expect(await screen.findByText('No requests right now.')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Shared with' })).not.toBeInTheDocument()
  })

  it.each([
    {
      name: 'shares',
      button: 'Share',
      question: 'Share your phone number with Darline?',
      yes: 'Yes, share',
      decision: 'share',
      after: [{ ...darline, status: 'shared' as const }],
      says: 'Shared with Darline.',
    },
    {
      name: 'quietly declines',
      button: 'Don’t share',
      question: 'Don’t share your phone number with Darline?',
      yes: 'Yes, don’t share',
      decision: 'decline',
      after: [],
      says: 'Not shared. Darline won’t be told.',
    },
  ])('$name a request once you say yes, and keeps it in place saying so', async ({ button, question, yes, decision, after, says }) => {
    const answers = {
      'GET /api/me/contact-requests': listed([darline]),
      [`POST /api/me/contact-requests/r1/${decision}`]: { status: 204 },
    }
    const { fetch, client } = renderCard(answers)

    fireEvent.click(await screen.findByRole('button', { name: button }))
    expect(posted(fetch, `/api/me/contact-requests/r1/${decision}`)).toBe(false)
    answers['GET /api/me/contact-requests'] = listed(after)
    fireEvent.click(within(screen.getByRole('group', { name: question })).getByRole('button', { name: yes }))

    expect(await screen.findByText(says)).toBeInTheDocument()
    expect(posted(fetch, `/api/me/contact-requests/r1/${decision}`)).toBe(true)
    await settled(client)
    expect(screen.getAllByText(/asked for your/).map((e) => e.textContent)).toEqual(['Darline Monestime asked for your phone number.'])
    expect(screen.getByText(says)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: button })).not.toBeInTheDocument()
    if (after.length) {
      expect(screen.getByRole('listitem')).toHaveTextContent('Darline Monestime · phone number')
    } else {
      expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    }
  })

  it.each([
    {
      button: 'Share',
      question: 'Share your phone number with Darline?',
      note: 'Darline will see every phone number you’ve set to “Family can ask”, including ones you add later.',
    },
    { button: 'Don’t share', question: 'Don’t share your phone number with Darline?', note: 'You can’t change your mind later.' },
  ])('asks before it answers “$button”, and Cancel sends nothing', async ({ button, question, note }) => {
    const { fetch } = renderCard({ 'GET /api/me/contact-requests': listed([darline]) })

    fireEvent.click(await screen.findByRole('button', { name: button }))
    const confirm = screen.getByRole('group', { name: question })
    expect(confirm).toHaveAccessibleDescription(note)
    fireEvent.click(within(confirm).getByRole('button', { name: 'Cancel' }))

    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Don’t share' })).toBeInTheDocument()
    expect(fetch.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(false)
  })

  it('asks before it stops sharing, then takes the share back', async () => {
    const answers = {
      'GET /api/me/contact-requests': listed([naika]),
      'POST /api/me/contact-requests/r3/revoke': { status: 204 },
    }
    const { fetch } = renderCard(answers)
    const confirm = () => within(screen.getByRole('group', { name: 'Stop sharing your WhatsApp number with Naïka?' }))

    fireEvent.click(await screen.findByRole('button', { name: 'Stop sharing' }))
    fireEvent.click(confirm().getByRole('button', { name: 'Keep sharing' }))
    expect(posted(fetch, '/api/me/contact-requests/r3/revoke')).toBe(false)

    fireEvent.click(screen.getByRole('button', { name: 'Stop sharing' }))
    answers['GET /api/me/contact-requests'] = listed([])
    fireEvent.click(confirm().getByRole('button', { name: 'Yes, stop' }))

    expect(await screen.findByText('Stopped sharing your WhatsApp number with Naïka.')).toBeInTheDocument()
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    expect(posted(fetch, '/api/me/contact-requests/r3/revoke')).toBe(true)
  })

  it('shows where a request stands once it was answered elsewhere', async () => {
    const answers: Record<string, Answer> = {
      'GET /api/me/contact-requests': listed([darline]),
      'POST /api/me/contact-requests/r1/share': { status: 409, body: { error: { code: 'conflict', message: 'x' } } },
    }
    renderCard(answers)

    fireEvent.click(await screen.findByRole('button', { name: 'Share' }))
    // Shared from another tab meanwhile.
    answers['GET /api/me/contact-requests'] = listed([{ ...darline, status: 'shared' }])
    fireEvent.click(screen.getByRole('button', { name: 'Yes, share' }))

    expect(await screen.findByRole('listitem')).toHaveTextContent('Darline Monestime · phone number')
    expect(screen.queryByRole('button', { name: 'Share' })).not.toBeInTheDocument()
  })
})
