import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { AccessRequestItem, Member } from '../../api/admin'
import { authKeys } from '../../api/auth'
import { personKeys } from '../../api/persons'
import { memberStatus } from './Everyone'
import { ManageAccess } from './ManageAccess'
import { relatedLine, requestedWhen } from './Requests'

const now = new Date('2026-11-20T12:00:00Z')

const member = (over: Partial<Member>): Member => ({
  id: 'u-fritz',
  email: 'fritz.p@example.com',
  role: 'member',
  person: { id: 'p-fritz', first_name: 'Fritz', last_name: 'Pluviose' },
  status: 'active',
  invite_expires_at: null,
  ...over,
})

const members: Member[] = [
  member({ id: 'u-maya', email: 'maya.pluviose@example.com', role: 'admin', person: { id: 'p-maya', first_name: 'Maya', last_name: 'Pluviose' } }),
  member({ id: 'u-nadege', email: 'nadege.p@example.com', role: 'admin', person: { id: 'p-nadege', first_name: 'Nadège', last_name: 'Pluviose' } }),
  member({}),
  member({
    id: 'u-sabine',
    email: 'sabine.p@example.com',
    person: { id: 'p-sabine', first_name: 'Sabine', last_name: 'Pluviose' },
    status: 'invited',
    invite_expires_at: '2026-11-25T12:00:00Z',
  }),
]

const accessRequest = (over: Partial<AccessRequestItem>): AccessRequestItem => ({
  id: 'ar-kervens',
  full_name: 'Kervens Pluviose',
  email: 'kervens.p@example.com',
  side: 'first',
  relationship: 'son',
  related_names: [
    { name: 'Fritz Pluviose', match: { person_id: 'p-fritz', name: 'Fritz Pluviose' } },
    { name: 'Fabiene Pluvios', match: null },
  ],
  note: 'Fritz told me about the site.',
  source: 'self',
  suggested_by: null,
  status: 'pending',
  reviewed_at: null,
  created_at: '2026-11-18T15:00:00Z',
  ...over,
})

const requests: AccessRequestItem[] = [
  accessRequest({}),
  accessRequest({
    id: 'ar-wesner',
    full_name: 'Wesner Jean-Louis',
    email: 'wesner.jl@example.com',
    source: 'member_suggestion',
    suggested_by: { person_id: 'p-fritz', name: 'Fritz Pluviose' },
    note: null,
    related_names: [],
  }),
  accessRequest({ id: 'ar-rose', full_name: 'Rose Monestime', email: 'rose.m@example.com', status: 'declined', note: null }),
]

const tree = {
  persons: [
    { id: 'p-fritz', first_name: 'Fritz', last_name: 'Pluviose', gender: 'male', is_deceased: false },
    { id: 'p-fabienne', first_name: 'Fabienne', last_name: 'Pluviose', gender: 'female', is_deceased: false },
    { id: 'p-kervens', first_name: 'Kervens', last_name: 'Pluviose', gender: 'male', is_deceased: false },
    { id: 'p-joseph', first_name: 'Joseph', last_name: 'Pluviose', gender: 'male', is_deceased: true },
  ],
  relationships: [
    { id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'p-fritz', child_id: 'p-kervens' },
    { id: 'r2', type: 'parent_child', subtype: 'biological', parent_id: 'p-fabienne', child_id: 'p-kervens' },
  ],
}

type Answer = { status: number; body?: unknown }

function stubApi(answers: Record<string, Answer>) {
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const answer = answers[`${init?.method ?? 'GET'} ${url}`]
    if (!answer) {
      return Promise.reject(new TypeError('Failed to fetch'))
    }
    return Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
  })
  vi.stubGlobal('fetch', fetch)
  return fetch
}

const listAnswers = {
  'GET /api/admin/users?limit=100': { status: 200, body: { users: members, next_cursor: null } },
  'GET /api/tree': { status: 200, body: tree },
  'GET /api/admin/access-requests': { status: 200, body: { requests } },
}

function renderPage(role: 'admin' | 'member' = 'admin', { personLoaded = true } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u-maya', email: 'maya.pluviose@example.com', role, person_id: 'p-maya' })
  if (personLoaded) {
    client.setQueryData(personKeys.one('p-maya'), { id: 'p-maya', first_name: 'Maya', last_name: 'Pluviose' })
  }
  const router = createMemoryRouter(
    [
      { path: '/admin/members', element: <ManageAccess /> },
      { path: '/', element: <p>Home page</p> },
    ],
    { initialEntries: ['/admin/members'] },
  )
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}

async function openInvite() {
  await screen.findByRole('tab', { name: 'Everyone (4)' })
  fireEvent.click(screen.getByRole('tab', { name: 'Invite someone' }))
}

const type = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } })

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('memberStatus', () => {
  it.each([
    { name: 'active', over: {}, want: 'Active' },
    { name: 'disabled', over: { status: 'disabled' as const }, want: 'Disabled' },
    { name: 'invited, days left', over: { status: 'invited' as const, invite_expires_at: '2026-11-25T11:00:00Z' }, want: 'Invited · expires in 5 days' },
    { name: 'invited, hours left', over: { status: 'invited' as const, invite_expires_at: '2026-11-20T13:00:00Z' }, want: 'Invited · expires in 1 day' },
    { name: 'invited, past', over: { status: 'invited' as const, invite_expires_at: '2026-11-19T12:00:00Z' }, want: 'Invited · expired' },
    { name: 'invited, no link left', over: { status: 'invited' as const }, want: 'Invited · expired' },
  ])('$name: $want', ({ over, want }) => {
    expect(memberStatus(member(over), now)).toBe(want)
  })
})

describe('requestedWhen', () => {
  it.each([
    { name: 'same day', created_at: '2026-11-20T08:00:00', source: 'self' as const, want: 'Requested today' },
    { name: 'the day before', created_at: '2026-11-19T23:00:00', source: 'member_suggestion' as const, want: 'Suggested yesterday' },
    { name: 'days before', created_at: '2026-11-18T01:00:00', source: 'self' as const, want: 'Requested 2 days ago' },
  ])('$name: $want', ({ created_at, source, want }) => {
    expect(requestedWhen(accessRequest({ created_at, source }), new Date('2026-11-20T12:00:00'))).toBe(want)
  })
})

describe('relatedLine', () => {
  it.each([
    { relationship: 'daughter', want: 'Daughter of Frantz Monestime' },
    { relationship: 'niece or nephew', want: 'Niece or nephew of Frantz Monestime' },
    { relationship: null, want: 'Frantz Monestime' },
  ])('$relationship: $want', ({ relationship, want }) => {
    expect(relatedLine(relationship, 'Frantz Monestime')).toBe(want)
  })
})

describe('ManageAccess', () => {
  it('sends a member home', async () => {
    stubApi(listAnswers)
    renderPage('member')

    expect(await screen.findByText('Home page')).toBeInTheDocument()
  })

  it('moves between tabs with the arrow keys', async () => {
    stubApi(listAnswers)
    renderPage()
    const first = await screen.findByRole('tab', { name: 'Requests (2)' })

    fireEvent.keyDown(first, { key: 'ArrowLeft' })

    const everyone = screen.getByRole('tab', { name: 'Everyone (4)' })
    expect(everyone).toHaveAttribute('aria-selected', 'true')
    expect(everyone).toHaveFocus()
    expect(first).toHaveAttribute('tabindex', '-1')
    expect(screen.getByRole('table')).toBeInTheDocument()
  })

  it('shows each request, with what the tree knows of the names', async () => {
    stubApi(listAnswers)
    renderPage()

    const first = await screen.findByRole('article', { name: 'Kervens Pluviose' })
    expect(screen.getByRole('tab', { name: 'Requests (2)' })).toHaveAttribute('aria-selected', 'true')
    expect(first).toHaveTextContent('Pluviose side')
    expect(first).toHaveTextContent('Asked on the sign-in page')
    expect(first).toHaveTextContent('Son of Fritz Pluviose✓ In the tree')
    expect(first).toHaveTextContent('Son of Fabiene PluviosNot found — check spelling')
    expect(first).toHaveTextContent('“Fritz told me about the site.”')
    const [, suggested, declined] = screen.getAllByRole('article')
    expect(suggested).toHaveTextContent('Suggested by Fritz Pluviose')
    expect(suggested).toHaveTextContent(/Suggested (today|yesterday|\d+ days ago)/)
    expect(within(declined!).getByRole('button', { name: 'Undo' })).toBeInTheDocument()
    expect(declined).toHaveTextContent('Set aside')
  })

  it.each([
    { name: 'sets a request aside', article: 'Kervens Pluviose', button: 'Not now', path: '/api/admin/access-requests/ar-kervens/decline' },
    { name: 'undoes a set-aside', article: 'Rose Monestime', button: 'Undo', path: '/api/admin/access-requests/ar-rose/reopen' },
  ])('$name, then rereads the queue', async ({ article, button, path }) => {
    const fetch = stubApi({ ...listAnswers, [`POST ${path}`]: { status: 204 } })
    renderPage()

    fireEvent.click(within(await screen.findByRole('article', { name: article })).getByRole('button', { name: button }))

    await vi.waitFor(() => expect(fetch).toHaveBeenCalledWith(path, expect.objectContaining({ method: 'POST' })))
    await vi.waitFor(() =>
      expect(fetch.mock.calls.filter(([url]) => url === '/api/admin/access-requests')).toHaveLength(2),
    )
  })

  it('says when another request from that email is already waiting', async () => {
    stubApi({
      ...listAnswers,
      'POST /api/admin/access-requests/ar-rose/reopen': { status: 409, body: { error: { code: 'conflict', message: 'x' } } },
    })
    renderPage()

    fireEvent.click(within(await screen.findByRole('article', { name: 'Rose Monestime' })).getByRole('button', { name: 'Undo' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('This request changed meanwhile, or a newer one from this email is waiting. Reload the page.')
  })

  it('approves a request through the invite form, filled in for them', async () => {
    const fetch = stubApi({ ...listAnswers, 'POST /api/admin/users': { status: 201, body: { id: 'u-kervens' } } })
    renderPage()
    await screen.findByRole('tab', { name: 'Everyone (4)' })

    fireEvent.click(
      within(screen.getByRole('article', { name: 'Kervens Pluviose' })).getByRole('button', { name: 'Approve & invite' }),
    )

    expect(screen.getByRole('tab', { name: 'Invite someone' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Invite someone' })).toHaveFocus()
    expect(screen.getByText('Selected')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveValue('kervens.p@example.com')
    expect(screen.getByRole('button', { name: /Maya Pluviose.*You/ })).toHaveAttribute('aria-pressed', 'true')

    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Invitation sent to Kervens, from Maya Pluviose')
    expect(fetch).toHaveBeenCalledWith(
      '/api/admin/users',
      expect.objectContaining({
        body: JSON.stringify({
          email: 'kervens.p@example.com',
          role: 'member',
          person_id: 'p-kervens',
          from_person_id: 'p-maya',
          access_request_id: 'ar-kervens',
        }),
      }),
    )
    await vi.waitFor(() =>
      expect(fetch.mock.calls.filter(([url]) => url === '/api/admin/access-requests')).toHaveLength(2),
    )
  })

  it('offers the member who suggested them as the sender, first and picked', async () => {
    stubApi(listAnswers)
    renderPage()
    await screen.findByRole('tab', { name: 'Everyone (4)' })

    fireEvent.click(within(screen.getAllByRole('article')[1]!).getByRole('button', { name: 'Approve & invite' }))

    const senders = within(screen.getByRole('group', { name: 'Invitation from' })).getAllByRole('button')
    expect(senders.map((b) => b.textContent)).toEqual([
      'FPFritz PluvioseSuggested Wesner',
      'MPMaya PluvioseYou',
      'NPNadège PluvioseAdmin',
    ])
    expect(senders[0]).toHaveAttribute('aria-pressed', 'true')
  })

  it('keeps your own label when you suggested them yourself', async () => {
    stubApi({
      ...listAnswers,
      'GET /api/admin/access-requests': {
        status: 200,
        body: { requests: [accessRequest({ source: 'member_suggestion', suggested_by: { person_id: 'p-maya', name: 'Maya Pluviose' } })] },
      },
    })
    renderPage()
    await screen.findByRole('tab', { name: 'Everyone (4)' })

    fireEvent.click(await screen.findByRole('button', { name: 'Approve & invite' }))

    const senders = within(screen.getByRole('group', { name: 'Invitation from' })).getAllByRole('button')
    expect(senders.map((b) => b.textContent)).toEqual(['MPMaya PluvioseYou', 'NPNadège PluvioseAdmin'])
  })

  it('keeps an approval filled in when an earlier invite finishes sending', async () => {
    let answer!: (r: Response) => void
    const fetch = stubApi(listAnswers)
    const listed = fetch.getMockImplementation()!
    fetch.mockImplementation((url: string, init?: RequestInit) =>
      url === '/api/admin/users' && init?.method === 'POST' ? new Promise<Response>((r) => (answer = r)) : listed(url, init),
    )
    renderPage()
    await openInvite()
    type('Find a person in the tree', 'fabi')
    fireEvent.click(await screen.findByRole('button', { name: /Fabienne Pluviose/ }))
    type('Email', 'fabienne.p@example.com')
    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))
    await vi.waitFor(() => expect(answer).toBeDefined())

    fireEvent.click(screen.getByRole('tab', { name: 'Requests (2)' }))
    fireEvent.click(
      within(screen.getByRole('article', { name: 'Kervens Pluviose' })).getByRole('button', { name: 'Approve & invite' }),
    )
    answer(new Response(JSON.stringify({ id: 'u-fabienne' }), { status: 201 }))

    await vi.waitFor(() =>
      expect(fetch.mock.calls.filter(([url]) => url === '/api/admin/access-requests')).toHaveLength(2),
    )
    expect(screen.getByLabelText('Email')).toHaveValue('kervens.p@example.com')
    expect(screen.getByText('Selected')).toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('doesn’t pick the requester over a search edited while the tree loads', async () => {
    let answerTree!: (r: Response) => void
    const fetch = stubApi(listAnswers)
    const listed = fetch.getMockImplementation()!
    fetch.mockImplementation((url: string, init?: RequestInit) =>
      url === '/api/tree' ? new Promise<Response>((r) => (answerTree = r)) : listed(url, init),
    )
    renderPage()
    await screen.findByRole('tab', { name: 'Everyone (4)' })
    fireEvent.click(
      within(screen.getByRole('article', { name: 'Kervens Pluviose' })).getByRole('button', { name: 'Approve & invite' }),
    )

    type('Find a person in the tree', 'fabi')
    answerTree(new Response(JSON.stringify(tree), { status: 200 }))

    expect(await screen.findByRole('button', { name: /Fabienne Pluviose/ })).toBeInTheDocument()
    expect(screen.queryByText('Selected')).not.toBeInTheDocument()
  })

  it('stops approving the request once the email is changed', async () => {
    const fetch = stubApi({ ...listAnswers, 'POST /api/admin/users': { status: 201, body: { id: 'u-kervens' } } })
    renderPage()
    await screen.findByRole('tab', { name: 'Everyone (4)' })
    fireEvent.click(
      within(screen.getByRole('article', { name: 'Kervens Pluviose' })).getByRole('button', { name: 'Approve & invite' }),
    )

    type('Email', 'someone.else@example.com')
    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    await screen.findByRole('status')
    expect(fetch).toHaveBeenCalledWith(
      '/api/admin/users',
      expect.objectContaining({ body: expect.not.stringContaining('access_request_id') }),
    )
  })

  it('lists everyone with one action each', async () => {
    stubApi(listAnswers)
    renderPage()

    fireEvent.click(await screen.findByRole('tab', { name: 'Everyone (4)' }))

    const rows = within(screen.getByRole('table')).getAllByRole('row').slice(1)
    expect(rows.map((row) => within(row).getAllByRole('cell')[0]!.textContent)).toEqual([
      'Maya Pluviose',
      'Nadège Pluviose',
      'Fritz Pluviose',
      'Sabine Pluviose',
    ])
    expect(within(rows[2]!).getByRole('button', { name: 'Send password reset to Fritz Pluviose' })).toBeInTheDocument()
    expect(within(rows[3]!).getByRole('button', { name: 'Resend invitation to Sabine Pluviose' })).toBeInTheDocument()
  })

  it.each([
    { name: 'sends the link', answer: { status: 204 }, want: 'Email sent' },
    {
      name: 'says when the mail failed',
      answer: { status: 502, body: { error: { code: 'mail_failed', message: 'x' } } },
      want: 'The email couldn’t be sent. Try again.',
    },
  ])('$name', async ({ answer, want }) => {
    stubApi({ ...listAnswers, 'POST /api/admin/users/u-sabine/send-reset': answer })
    renderPage()
    fireEvent.click(await screen.findByRole('tab', { name: 'Everyone (4)' }))

    fireEvent.click(screen.getByRole('button', { name: 'Resend invitation to Sabine Pluviose' }))

    expect(await screen.findByText(want)).toBeInTheDocument()
  })

  it.each([
    { name: 'sent', answer: { status: 204 } },
    { name: 'mail failed', answer: { status: 502, body: { error: { code: 'mail_failed', message: 'x' } } } },
  ])('rereads the list after a resend ($name), so the new expiry shows', async ({ answer }) => {
    const fetch = stubApi({ ...listAnswers, 'POST /api/admin/users/u-sabine/send-reset': answer })
    renderPage()
    fireEvent.click(await screen.findByRole('tab', { name: 'Everyone (4)' }))

    fireEvent.click(screen.getByRole('button', { name: 'Resend invitation to Sabine Pluviose' }))

    await vi.waitFor(() =>
      expect(fetch.mock.calls.filter(([url]) => url === '/api/admin/users?limit=100')).toHaveLength(2),
    )
  })

  it('finds a living person in the tree and marks who already has a login', async () => {
    stubApi(listAnswers)
    renderPage()
    await openInvite()

    type('Find a person in the tree', 'plu')

    const matches = within(await screen.findByRole('list', { name: 'People in the tree' })).getAllByRole('button')
    expect(matches.map((m) => m.textContent)).toEqual([
      'FPFabienne PluvioseIn the tree',
      'FPFritz PluvioseAlready has a login',
      'KPKervens PluvioseIn the tree as Fritz and Fabienne’s son',
    ])
    expect(matches[1]).toHaveAttribute('aria-disabled', 'true')
  })

  it('invites them from whoever is picked, then says so', async () => {
    const fetch = stubApi({ ...listAnswers, 'POST /api/admin/users': { status: 201, body: member({ id: 'u-kervens' }) } })
    renderPage()
    await openInvite()

    type('Find a person in the tree', 'kerv')
    fireEvent.click(await screen.findByRole('button', { name: /Kervens Pluviose/ }))
    type('Email', 'kervens.p@example.com')
    fireEvent.click(screen.getByRole('button', { name: /Admin.*Can also edit/ }))
    fireEvent.click(screen.getByRole('button', { name: /Nadège Pluviose/ }))

    const preview = screen.getByRole('complementary', { name: 'Invitation preview' })
    expect(preview).toHaveTextContent('Subject: Nadège invited you to the family hub')
    expect(preview).toHaveTextContent('Bonjou Kervens,')
    expect(preview).toHaveTextContent('Approved by Maya Pluviose, admin.')

    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Invitation sent to Kervens, from Nadège Pluviose')
    expect(fetch).toHaveBeenCalledWith('/api/admin/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'kervens.p@example.com', role: 'admin', person_id: 'p-kervens', from_person_id: 'p-nadege' }),
      signal: undefined,
    })
  })

  it('still sends in your name when your own person couldn’t be read', async () => {
    stubApi(listAnswers)
    renderPage('admin', { personLoaded: false })
    await openInvite()

    expect(screen.getByRole('button', { name: /Maya Pluviose.*You/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('complementary', { name: 'Invitation preview' })).toHaveTextContent(
      'Subject: Maya invited you to the family hub',
    )
  })

  it('keeps an invite that’s still sending across a tab switch', async () => {
    let answer!: (r: Response) => void
    const fetch = stubApi(listAnswers)
    const listed = fetch.getMockImplementation()!
    fetch.mockImplementation((url: string, init?: RequestInit) =>
      url === '/api/admin/users' && init?.method === 'POST' ? new Promise<Response>((r) => (answer = r)) : listed(url, init),
    )
    renderPage()
    await openInvite()
    type('Find a person in the tree', 'kerv')
    fireEvent.click(await screen.findByRole('button', { name: /Kervens Pluviose/ }))
    type('Email', 'kervens.p@example.com')
    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    fireEvent.click(screen.getByRole('tab', { name: 'Everyone (4)' }))
    await vi.waitFor(() => expect(answer).toBeDefined())
    answer(new Response(JSON.stringify({ id: 'u-kervens' }), { status: 201 }))
    fireEvent.click(screen.getByRole('tab', { name: 'Invite someone' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Invitation sent to Kervens, from Maya Pluviose')
  })

  it('asks for the person before sending', async () => {
    const fetch = stubApi(listAnswers)
    renderPage()
    await openInvite()
    type('Email', 'kervens.p@example.com')

    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Pick who you’re inviting first.')
    expect(fetch).not.toHaveBeenCalledWith('/api/admin/users', expect.anything())
  })

  it.each([
    { code: 'email_taken', status: 409, want: 'That email already has a login.' },
    { code: 'person_taken', status: 409, want: 'Kervens already has a login.' },
    { code: 'mail_failed', status: 502, want: 'The email couldn’t be sent, so nothing was saved. Try again.' },
    { code: 'conflict', status: 409, want: 'Another admin already handled this request. Reload the page.' },
  ])('explains $code', async ({ code, status, want }) => {
    stubApi({ ...listAnswers, 'POST /api/admin/users': { status, body: { error: { code, message: 'x' } } } })
    renderPage()
    await openInvite()
    type('Find a person in the tree', 'kerv')
    fireEvent.click(await screen.findByRole('button', { name: /Kervens Pluviose/ }))
    type('Email', 'kervens.p@example.com')

    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(want)
  })
})
