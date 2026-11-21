import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { AppShell } from './AppShell'

const maya = { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: 'p1' }
const mayaPerson = { id: 'p1', first_name: 'Maya', last_name: 'Pluviose' }

type Answer = { status: number; body?: unknown } | 'network error'

// Answers each API path with its own canned reply.
function stubApi(answers: Record<string, Answer>) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      const answer = answers[url]
      if (answer === undefined || answer === 'network error') {
        return Promise.reject(new TypeError('Failed to fetch'))
      }
      return Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
    }),
  )
}

function renderShell() {
  const router = createMemoryRouter(
    [
      { element: <AppShell />, children: [{ path: '/', element: <main>Home page</main> }] },
      { path: '/auth/login', element: <p>Sign-in page</p> },
    ],
    { initialEntries: ['/'] },
  )
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('AppShell', () => {
  it('shows the menu around the page, with the current page marked', async () => {
    stubApi({ '/api/auth/me': { status: 200, body: maya }, '/api/persons/p1': { status: 200, body: mayaPerson } })
    renderShell()

    const menu = await screen.findByRole('navigation', { name: 'Main' })
    expect(within(menu).getAllByRole('link').map((link) => link.textContent)).toEqual([
      'Pluviose-MonestimeFamily Hub',
      'Home',
      'Family tree',
      'Members',
      'Events',
      'Stories',
      'My profile',
      'Members & invites',
    ])
    expect(within(menu).getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page')
    expect(within(menu).getByText('Signed in as Maya')).toBeInTheDocument()
    expect(screen.getByText('Home page')).toBeInTheDocument()
  })

  it.each([
    {
      name: 'a member sees no admin section',
      me: { ...maya, role: 'member' },
      absent: 'Members & invites',
      signedInAs: 'Signed in as Maya',
    },
    {
      name: 'a login linked to nobody shows its email',
      me: { ...maya, person_id: null },
      absent: undefined,
      signedInAs: 'Signed in as maya.pluviose@example.com',
    },
  ])('$name', async ({ me, absent, signedInAs }) => {
    stubApi({ '/api/auth/me': { status: 200, body: me }, '/api/persons/p1': { status: 200, body: mayaPerson } })
    renderShell()

    expect(await screen.findByText(signedInAs)).toBeInTheDocument()
    if (absent) {
      expect(screen.queryByRole('link', { name: absent })).not.toBeInTheDocument()
    }
  })

  it.each([
    { name: 'counts only the requests waiting for an answer', statuses: ['pending', 'shared', 'pending'], link: 'My profile, 2 requests for your details' },
    { name: 'says one in the singular', statuses: ['pending'], link: 'My profile, 1 request for your details' },
  ])('badge on My profile: $name', async ({ statuses, link }) => {
    const requests = statuses.map((status, i) => ({
      id: `r${i}`,
      kind: 'phone',
      status,
      created_at: '2026-11-01T10:00:00Z',
      decided_at: null,
      from: { person_id: 'p2', name: 'Darline Monestime' },
    }))
    stubApi({
      '/api/auth/me': { status: 200, body: maya },
      '/api/persons/p1': { status: 200, body: mayaPerson },
      '/api/me/contact-requests': { status: 200, body: { person_id: 'p1', requests } },
    })
    renderShell()

    expect(await screen.findByRole('link', { name: link })).toHaveAttribute('href', '/profile')
  })

  it('shows a new request on the badge within a minute, without a reload', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const asked = (id: string) => ({
      id,
      kind: 'phone',
      status: 'pending',
      created_at: '2026-11-01T10:00:00Z',
      decided_at: null,
      from: { person_id: 'p2', name: 'Darline Monestime' },
    })
    const answers: Record<string, Answer> = {
      '/api/auth/me': { status: 200, body: maya },
      '/api/persons/p1': { status: 200, body: mayaPerson },
      '/api/me/contact-requests': { status: 200, body: { person_id: 'p1', requests: [asked('r1')] } },
    }
    stubApi(answers)
    renderShell()
    await screen.findByRole('link', { name: 'My profile, 1 request for your details' })

    answers['/api/me/contact-requests'] = { status: 200, body: { person_id: 'p1', requests: [asked('r1'), asked('r2')] } }
    await act(() => vi.advanceTimersByTimeAsync(60_000))

    expect(await screen.findByRole('link', { name: 'My profile, 2 requests for your details' })).toBeInTheDocument()
  })

  it('asks for no requests for a login linked to nobody', async () => {
    stubApi({ '/api/auth/me': { status: 200, body: { ...maya, person_id: null } } })
    renderShell()

    expect(await screen.findByText('Signed in as maya.pluviose@example.com')).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalledWith('/api/me/contact-requests', expect.anything())
  })

  it('names the login by its email when the person can’t be read', async () => {
    stubApi({ '/api/auth/me': { status: 200, body: maya }, '/api/persons/p1': { status: 500 } })
    renderShell()

    expect(await screen.findByText('Signed in as maya.pluviose@example.com')).toBeInTheDocument()
  })

  it('sends someone signed out to sign in', async () => {
    stubApi({ '/api/auth/me': { status: 401, body: { error: { code: 'unauthorized', message: 'sign in first' } } } })
    renderShell()

    expect(await screen.findByText('Sign-in page')).toBeInTheDocument()
  })

  it('says so when the server can’t be reached', async () => {
    stubApi({ '/api/auth/me': 'network error' })
    renderShell()

    expect(await screen.findByRole('alert')).toHaveTextContent('Can’t reach the family hub. Check your connection and try again.')
  })
})
