import { QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys, followSessionChanges, type LinkPreview } from '../../api/auth'
import { createQueryClient } from '../../api/queryClient'
import { WelcomeInvite } from './WelcomeInvite'

const invite: LinkPreview = {
  purpose: 'invite',
  invitee_email: 'darline.monestime@example.com',
  invitee_first_name: 'Darline',
  from_name: 'Nadège Pluviose',
}
const reset: LinkPreview = { ...invite, purpose: 'password_reset', invitee_first_name: null, from_name: null }
const invalidToken = () => Response.json({ error: { code: 'invalid_token', message: 'x' } }, { status: 400 })
const user = { id: '01890000-0000-7000-8000-000000000001', email: invite.invitee_email, role: 'member' }

type Answers = { preview?: Response; reset?: Response }

function renderPage(answers: Answers, url = '/auth/set-password#t=link-token') {
  const fetch = vi.fn((path: string) => {
    switch (path) {
      case '/api/auth/invite/preview':
        return Promise.resolve(answers.preview ?? Response.json(invite))
      default:
        return Promise.resolve(answers.reset ?? Response.json(user))
    }
  })
  vi.stubGlobal('fetch', fetch)
  const client = createQueryClient()
  const router = createMemoryRouter(
    [
      { path: '/auth/set-password', element: <WelcomeInvite /> },
      { path: '/setup', element: <p>Setup page</p> },
      { path: '/', element: <p>Home page</p> },
    ],
    { initialEntries: [url] },
  )
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { fetch, router, client }
}

function choose(password: string) {
  fireEvent.change(screen.getByLabelText('New password'), { target: { value: password } })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('WelcomeInvite', () => {
  it.each([
    { name: 'no token in the link', url: '/auth/set-password', answers: {}, why: 'Open it again from your email.' },
    { name: 'a used or expired link', url: undefined, answers: { preview: invalidToken() }, why: 'It may have expired or already been used.' },
  ])('explains $name', async ({ url, answers, why }) => {
    renderPage(answers, url)

    expect(await screen.findByRole('heading', { name: 'This link doesn’t work anymore' })).toBeInTheDocument()
    expect(screen.getByText(why)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Get a new reset link' })).toHaveAttribute('href', '/auth/forgot')
  })

  it('welcomes an invitee by name, with who invited them', async () => {
    renderPage({})

    expect(await screen.findByRole('heading', { name: 'Welcome to the family, Darline' })).toBeInTheDocument()
    expect(screen.getByText('Nadège Pluviose')).toBeInTheDocument()
    expect(screen.getByText(/This link works for 7 days\. Not Darline\?/)).toBeInTheDocument()
    expect(screen.getByLabelText('New password')).toHaveAttribute('type', 'text')
  })

  it('takes the token off the address bar once read', async () => {
    const { router } = renderPage({})

    expect(await screen.findByLabelText('New password')).toBeInTheDocument()
    expect(router.state.location.hash).toBe('')
  })

  it('reads a new link pasted into the same tab', async () => {
    const { router } = renderPage({}, '/auth/set-password')
    await screen.findByRole('heading', { name: 'This link doesn’t work anymore' })

    await act(() => router.navigate('/auth/set-password#t=new-token'))

    expect(await screen.findByLabelText('New password')).toBeInTheDocument()
    expect(router.state.location.hash).toBe('')
  })

  it('gives a failed link check a heading and a way to try again', async () => {
    renderPage({ preview: Response.json({ error: { code: 'rate_limited', message: 'x' } }, { status: 429 }) })

    expect(await screen.findByRole('heading', { name: 'Welcome' })).toBeInTheDocument()
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })

  it('drops the token when the address changes to one without a link', async () => {
    const { router } = renderPage({})
    await screen.findByLabelText('New password')

    await act(() => router.navigate('/auth/set-password'))

    expect(await screen.findByText('Open it again from your email.')).toBeInTheDocument()
  })

  it('announces a refusal again when the same short password is resubmitted', async () => {
    renderPage({})
    await screen.findByLabelText('New password')
    choose('kafe')
    fireEvent.click(screen.getByRole('button', { name: 'Set password and continue' }))
    const first = screen.getByRole('alert')

    fireEvent.click(screen.getByRole('button', { name: 'Set password and continue' }))

    expect(screen.getByRole('alert')).not.toBe(first)
  })

  it('welcomes an invitee without names yet', async () => {
    renderPage({ preview: Response.json({ ...invite, invitee_first_name: null, from_name: null }) })

    expect(await screen.findByRole('heading', { name: 'Welcome to the family' })).toBeInTheDocument()
    expect(screen.queryByText(/invited you to the family hub/)).not.toBeInTheDocument()
    expect(screen.getByText(/Not you\?/)).toBeInTheDocument()
  })

  it('asks for a new password on a reset link', async () => {
    renderPage({ preview: Response.json(reset) })

    expect(await screen.findByRole('heading', { name: 'Choose a new password' })).toBeInTheDocument()
    expect(screen.getByText(/This link works for 30 minutes\./)).toBeInTheDocument()
  })

  it.each([
    { name: 'a short password', password: 'kafe', rule: 'Use at least 8 characters.' },
    { name: 'an empty field', password: '', rule: 'Use at least 8 characters.' },
    { name: 'a password over the limit', password: 'k'.repeat(129), rule: 'Use at most 128 characters.' },
  ])('refuses $name before asking the server', async ({ password, rule }) => {
    const { fetch } = renderPage({})
    await screen.findByLabelText('New password')

    choose(password)
    fireEvent.click(screen.getByRole('button', { name: 'Set password and continue' }))

    expect(screen.getByRole('alert')).toHaveTextContent(rule)
    expect(screen.getByText(rule)).toHaveAttribute('id', 'wi-pw-error')
    expect(screen.getByLabelText('New password')).toHaveAttribute('aria-describedby', 'wi-pw-error')
    expect(fetch).not.toHaveBeenCalledWith('/api/auth/password/reset', expect.anything())
  })

  it('shows a password the server refuses under the field', async () => {
    renderPage({ reset: Response.json({ error: { code: 'password_too_common', message: 'x' } }, { status: 400 }) })
    await screen.findByLabelText('New password')

    choose('password123')
    fireEvent.click(screen.getByRole('button', { name: 'Set password and continue' }))

    expect(await screen.findByText('That password is too common — try a short phrase.')).toBeInTheDocument()
    expect(screen.getByLabelText('New password')).toHaveAttribute('aria-invalid', 'true')
  })

  it.each([
    { name: 'an invitee goes on to setup', preview: invite, button: 'Set password and continue', next: 'Setup page' },
    { name: 'a reset goes home', preview: reset, button: 'Set password and sign in', next: 'Home page' },
  ])('$name', async ({ preview, button, next }) => {
    const { fetch, client } = renderPage({ preview: Response.json(preview) })
    await screen.findByLabelText('New password')
    // Left by whoever used this browser before.
    client.setQueryData(['persons', 'nadege'], { cached: true })

    choose('mango tree on rue sainte-anne')
    fireEvent.click(screen.getByRole('button', { name: button }))

    expect(await screen.findByText(next)).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/auth/password/reset', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: 'link-token', password: 'mango tree on rue sainte-anne' }),
    })
    expect(client.getQueryData(['persons', 'nadege'])).toBeUndefined()
    expect(client.getQueryData(authKeys.me)).toEqual(user)
  })

  it('explains a link that stopped working while the page was open', async () => {
    renderPage({ reset: invalidToken() })
    await screen.findByLabelText('New password')

    choose('mango tree on rue sainte-anne')
    fireEvent.click(screen.getByRole('button', { name: 'Set password and continue' }))

    expect(await screen.findByRole('heading', { name: 'This link doesn’t work anymore' })).toHaveFocus()
    expect(document.title).toBe('This link doesn’t work anymore — Pluviose Family Hub')
  })

  it('keeps a half-typed password when someone signs in from another tab', async () => {
    const { fetch, client } = renderPage({})
    const stop = followSessionChanges(client)
    await screen.findByLabelText('New password')
    choose('mango tree on rue sainte-anne')

    act(() => {
      window.dispatchEvent(new StorageEvent('storage', { key: 'fh-session-changed', newValue: '1' }))
    })

    await act(() => new Promise((resolve) => setTimeout(resolve, 0)))
    expect(screen.getByLabelText('New password')).toHaveValue('mango tree on rue sainte-anne')
    // The preview endpoint is rate-limited, so the link isn't checked a second time either.
    expect(fetch.mock.calls.filter(([path]) => path === '/api/auth/invite/preview')).toHaveLength(1)
    stop()
  })
})
