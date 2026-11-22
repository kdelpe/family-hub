import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
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
      'PluvioseFamily Hub',
      'Home',
      'Family tree',
      'Members',
      'Events',
      'Stories',
      'Members & invites',
    ])
    expect(within(menu).getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByText('Home page')).toBeInTheDocument()
  })

  it.each([
    { name: 'a member sees no admin section', me: { ...maya, role: 'member' }, absent: 'Members & invites', initials: 'MP', named: 'Maya Pluviose' },
    { name: 'a login linked to nobody is named by its email', me: { ...maya, person_id: null }, absent: undefined, initials: 'M', named: 'maya.pluviose@example.com' },
  ])('$name', async ({ me, absent, initials, named }) => {
    stubApi({ '/api/auth/me': { status: 200, body: me }, '/api/persons/p1': { status: 200, body: mayaPerson } })
    renderShell()

    const picture = await screen.findByRole('button', { name: 'Your account' })
    expect(picture).toHaveTextContent(initials)
    fireEvent.click(picture)
    const panel = within(screen.getByRole('dialog', { name: 'Your account' }))
    expect(panel.getByText(named)).toBeInTheDocument()
    // Only someone in the tree can suggest a relative.
    expect(panel.queryByRole('link', { name: 'Suggest a relative' }) !== null).toBe(me.person_id !== null)
    if (absent) {
      expect(screen.queryByRole('link', { name: absent })).not.toBeInTheDocument()
    }
  })

  it.each([
    { name: 'counts only the requests waiting for an answer', statuses: ['pending', 'shared', 'pending'], label: 'Your account, 2 requests waiting' },
    { name: 'says one in the singular', statuses: ['pending'], label: 'Your account, 1 request waiting' },
  ])('count on the account picture: $name', async ({ statuses, label }) => {
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

    fireEvent.click(await screen.findByRole('button', { name: label }))

    expect(screen.getByRole('link', { name: `Requests, ${label.replace('Your account, ', '')}` })).toHaveAttribute('href', '/requests')
  })

  it('shows a new request on the picture within a minute, without a reload', async () => {
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
    await screen.findByRole('button', { name: 'Your account, 1 request waiting' })

    answers['/api/me/contact-requests'] = { status: 200, body: { person_id: 'p1', requests: [asked('r1'), asked('r2')] } }
    await act(() => vi.advanceTimersByTimeAsync(60_000))

    expect(await screen.findByRole('button', { name: 'Your account, 2 requests waiting' })).toBeInTheDocument()
  })

  it('asks for no requests for a login linked to nobody', async () => {
    stubApi({ '/api/auth/me': { status: 200, body: { ...maya, person_id: null } } })
    renderShell()

    expect(await screen.findByRole('button', { name: 'Your account' })).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalledWith('/api/me/contact-requests', expect.anything())
  })

  it('opens the account panel with its items, and Esc closes it back onto the picture', async () => {
    stubApi({ '/api/auth/me': { status: 200, body: maya }, '/api/persons/p1': { status: 200, body: mayaPerson } })
    renderShell()
    const picture = await screen.findByRole('button', { name: 'Your account' })

    fireEvent.click(picture)
    const panel = within(screen.getByRole('dialog', { name: 'Your account' }))
    expect(panel.getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual(['/profile', '/requests', '/account#suggest', '/account#password'])
    expect(panel.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
    expect(panel.getByRole('link', { name: 'View my profile ›' })).toHaveFocus()

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(picture).toHaveFocus()
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
