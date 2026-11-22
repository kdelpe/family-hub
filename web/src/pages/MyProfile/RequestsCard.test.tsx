import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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
  vi.useRealTimers()
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
    fireEvent.click(within(screen.getByRole('dialog', { name: question })).getByRole('button', { name: yes }))

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
    const confirm = screen.getByRole('dialog', { name: question })
    expect(confirm).toHaveAccessibleDescription(note)
    fireEvent.click(within(confirm).getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Don’t share' })).toBeInTheDocument()
    expect(fetch.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(false)
  })

  it('keeps keyboard focus in place through the question, Cancel and an answer', async () => {
    const answers = {
      'GET /api/me/contact-requests': listed([darline]),
      'POST /api/me/contact-requests/r1/decline': { status: 204 },
    }
    renderCard(answers)

    const decline = await screen.findByRole('button', { name: 'Don’t share' })
    decline.focus()
    fireEvent.click(decline)
    expect(screen.getByRole('button', { name: 'Yes, don’t share' })).toHaveFocus()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Don’t share' })).toHaveFocus())

    fireEvent.click(screen.getByRole('button', { name: 'Don’t share' }))
    answers['GET /api/me/contact-requests'] = listed([])
    fireEvent.click(screen.getByRole('button', { name: 'Yes, don’t share' }))
    const said = await screen.findByText('Not shared. Darline won’t be told.')
    await waitFor(() => expect(said).toHaveFocus())
  })

  it('keeps keyboard focus in place when you stop sharing or keep sharing', async () => {
    const answers = {
      'GET /api/me/contact-requests': listed([naika]),
      'POST /api/me/contact-requests/r3/revoke': { status: 204 },
    }
    renderCard(answers)

    const stop = await screen.findByRole('button', { name: 'Stop sharing' })
    stop.focus()
    fireEvent.click(stop)
    expect(screen.getByRole('button', { name: 'Yes, stop' })).toHaveFocus()
    fireEvent.click(screen.getByRole('button', { name: 'Keep sharing' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Stop sharing' })).toHaveFocus())

    fireEvent.click(screen.getByRole('button', { name: 'Stop sharing' }))
    answers['GET /api/me/contact-requests'] = listed([])
    fireEvent.click(screen.getByRole('button', { name: 'Yes, stop' }))
    const said = await screen.findByText('Stopped sharing your WhatsApp number with Naïka.')
    await waitFor(() => expect(said).toHaveFocus())
  })

  it('asks before it stops sharing, then takes the share back', async () => {
    const answers = {
      'GET /api/me/contact-requests': listed([naika]),
      'POST /api/me/contact-requests/r3/revoke': { status: 204 },
    }
    const { fetch } = renderCard(answers)
    const confirm = () => within(screen.getByRole('dialog', { name: 'Stop sharing your WhatsApp number with Naïka?' }))

    fireEvent.click(await screen.findByRole('button', { name: 'Stop sharing' }))
    fireEvent.click(confirm().getByRole('button', { name: 'Keep sharing' }))
    expect(posted(fetch, '/api/me/contact-requests/r3/revoke')).toBe(false)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Stop sharing' }))
    answers['GET /api/me/contact-requests'] = listed([])
    fireEvent.click(confirm().getByRole('button', { name: 'Yes, stop' }))

    expect(await screen.findByText('Stopped sharing your WhatsApp number with Naïka.')).toBeInTheDocument()
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    expect(posted(fetch, '/api/me/contact-requests/r3/revoke')).toBe(true)
  })

  it('hides the rows, so none can be answered, when a reread is for someone else and who you are can’t be read', async () => {
    const answers: Record<string, Answer> = { 'GET /api/me/contact-requests': listed([darline]) }
    const { client } = renderCard(answers)
    await screen.findByRole('button', { name: 'Share' })
    answers['GET /api/me/contact-requests'] = { status: 200, body: { person_id: 'fritz', requests: [darline] } }

    act(() => void client.invalidateQueries({ queryKey: ['contact-requests', 'maya'] }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong')
    expect(screen.queryByRole('button', { name: 'Share' })).not.toBeInTheDocument()
  })

  it('keeps your rows when a reread for someone else was a blip and the next one is yours', async () => {
    const answers: Record<string, Answer> = { 'GET /api/me/contact-requests': listed([darline]) }
    const { client, fetch } = renderCard(answers)
    await screen.findByRole('button', { name: 'Share' })
    let reads = 0
    const yours = fetch.getMockImplementation()!
    fetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/me/contact-requests' && reads++ === 0) {
        return Promise.resolve(new Response(JSON.stringify({ person_id: 'fritz', requests: [] }), { status: 200 }))
      }
      return yours(url, init)
    })

    act(() => void client.invalidateQueries({ queryKey: ['contact-requests', 'maya'] }))
    await settled(client)

    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('no longer says “Shared with” once a share given on this visit is taken back', async () => {
    const answers: Record<string, Answer> = {
      'GET /api/me/contact-requests': listed([darline]),
      'POST /api/me/contact-requests/r1/share': { status: 204 },
      'POST /api/me/contact-requests/r1/revoke': { status: 204 },
    }
    const { client } = renderCard(answers)
    fireEvent.click(await screen.findByRole('button', { name: 'Share' }))
    answers['GET /api/me/contact-requests'] = listed([{ ...darline, status: 'shared' }])
    fireEvent.click(screen.getByRole('button', { name: 'Yes, share' }))
    await screen.findByText('Shared with Darline.')
    await settled(client)

    fireEvent.click(screen.getByRole('button', { name: 'Stop sharing' }))
    answers['GET /api/me/contact-requests'] = listed([])
    fireEvent.click(screen.getByRole('button', { name: 'Yes, stop' }))

    expect(await screen.findByText('Stopped sharing your phone number with Darline.')).toBeInTheDocument()
    expect(screen.queryByText('Shared with Darline.')).not.toBeInTheDocument()
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
