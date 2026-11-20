import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { Member } from '../../api/admin'
import { authKeys } from '../../api/auth'
import { personKeys } from '../../api/persons'
import { memberStatus } from './Everyone'
import { ManageAccess } from './ManageAccess'

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

describe('ManageAccess', () => {
  it('sends a member home', async () => {
    stubApi(listAnswers)
    renderPage('member')

    expect(await screen.findByText('Home page')).toBeInTheDocument()
  })

  it('moves between tabs with the arrow keys', async () => {
    stubApi(listAnswers)
    renderPage()
    const invite = await screen.findByRole('tab', { name: 'Invite someone' })

    fireEvent.keyDown(invite, { key: 'ArrowRight' })

    const everyone = screen.getByRole('tab', { name: 'Everyone (4)' })
    expect(everyone).toHaveAttribute('aria-selected', 'true')
    expect(everyone).toHaveFocus()
    expect(invite).toHaveAttribute('tabindex', '-1')
    expect(screen.getByRole('table')).toBeInTheDocument()
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
    await screen.findByRole('tab', { name: 'Everyone (4)' })

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
    await screen.findByRole('tab', { name: 'Everyone (4)' })

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
    await screen.findByRole('tab', { name: 'Everyone (4)' })

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
    await screen.findByRole('tab', { name: 'Everyone (4)' })
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
    await screen.findByRole('tab', { name: 'Everyone (4)' })
    type('Email', 'kervens.p@example.com')

    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Pick who you’re inviting first.')
    expect(fetch).not.toHaveBeenCalledWith('/api/admin/users', expect.anything())
  })

  it.each([
    { code: 'email_taken', status: 409, want: 'That email already has a login.' },
    { code: 'person_taken', status: 409, want: 'Kervens already has a login.' },
    { code: 'mail_failed', status: 502, want: 'The email couldn’t be sent, so nothing was saved. Try again.' },
  ])('explains $code', async ({ code, status, want }) => {
    stubApi({ ...listAnswers, 'POST /api/admin/users': { status, body: { error: { code, message: 'x' } } } })
    renderPage()
    await screen.findByRole('tab', { name: 'Everyone (4)' })
    type('Find a person in the tree', 'kerv')
    fireEvent.click(await screen.findByRole('button', { name: /Kervens Pluviose/ }))
    type('Email', 'kervens.p@example.com')

    fireEvent.click(screen.getByRole('button', { name: 'Send invitation' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(want)
  })
})
