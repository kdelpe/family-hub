import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { Login } from './Login'

const user = { id: '01890000-0000-7000-8000-000000000001', email: 'maya.pluviose@example.com', role: 'admin' }

type Answer = { status: number; body?: unknown; headers?: Record<string, string> } | 'network error'

function stubApi(answers: { providers?: Answer; login?: Answer }) {
  const fetch = vi.fn((path: string) => {
    const answer = path === '/api/auth/providers' ? answers.providers : answers.login
    if (answer === undefined || answer === 'network error') {
      return Promise.reject(new TypeError('Failed to fetch'))
    }
    return Promise.resolve(
      new Response(answer.body === undefined ? null : JSON.stringify(answer.body), {
        status: answer.status,
        headers: answer.headers,
      }),
    )
  })
  vi.stubGlobal('fetch', fetch)
  return fetch
}

function renderLogin(url = '/auth/login') {
  const router = createMemoryRouter(
    [
      { path: '/auth/login', element: <Login /> },
      { path: '/', element: <p>Home page</p> },
    ],
    { initialEntries: [url] },
  )
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const providersSettled = () =>
    vi.waitFor(() => expect(client.getQueryState(authKeys.providers)?.status).not.toBe('pending'))
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { providersSettled }
}

function signIn() {
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: user.email } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'kafe-ak-pen-patat' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
}

const noGoogle: Answer = { status: 200, body: { google: false } }

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Login', () => {
  it.each([
    { name: 'shows Google when it is configured', google: true },
    { name: 'hides Google when it is not', google: false },
  ])('$name', async ({ google }) => {
    stubApi({ providers: { status: 200, body: { google } } })
    const { providersSettled } = renderLogin()

    expect(screen.getByRole('heading', { name: 'Lakay se lakay' })).toBeInTheDocument()
    expect(screen.getByText('Already invited? Use the link in your invitation email.')).toBeInTheDocument()
    if (google) {
      expect(await screen.findByRole('link', { name: 'Continue with Google' })).toHaveAttribute(
        'href',
        '/api/auth/google/start',
      )
    } else {
      await providersSettled()
      expect(screen.queryByRole('link', { name: 'Continue with Google' })).not.toBeInTheDocument()
    }
  })

  it('shows and hides the password', async () => {
    stubApi({ providers: noGoogle })
    const { providersSettled } = renderLogin()
    const toggle = screen.getByRole('button', { name: 'Show' })

    fireEvent.click(toggle)

    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'text')
    expect(screen.getByRole('button', { name: 'Hide' })).toHaveAttribute('aria-pressed', 'true')
    await providersSettled()
  })

  it('signs in and goes home', async () => {
    const fetch = stubApi({ providers: noGoogle, login: { status: 200, body: user } })
    renderLogin()

    signIn()

    expect(await screen.findByText('Home page')).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: user.email, password: 'kafe-ak-pen-patat' }),
    })
  })

  it('keeps a pending sign-in when the user types', async () => {
    const fetch = stubApi({ providers: noGoogle })
    fetch.mockImplementation((path: string) =>
      path === '/api/auth/providers'
        ? Promise.resolve(Response.json({ google: false }))
        : new Promise<Response>(() => {}),
    )
    const { providersSettled } = renderLogin()
    await providersSettled()

    signIn()
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'typo@example.com' } })

    expect(screen.getByRole('button', { name: 'Signing in…' })).toBeDisabled()
  })

  it('clears a shown error once the user types', async () => {
    stubApi({ providers: noGoogle, login: { status: 401, body: { error: { code: 'invalid_credentials', message: 'x' } } } })
    renderLogin()
    signIn()
    await screen.findByRole('alert')

    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'kafe-ak-pen-patat!' } })

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it.each([
    {
      name: 'wrong email or password',
      login: { status: 401, body: { error: { code: 'invalid_credentials', message: 'x' } } },
      want: 'That email and password don’t match. Check them and try again, or use Forgot password.',
    },
    {
      name: 'too many tries',
      login: { status: 429, body: { error: { code: 'rate_limited', message: 'x' } }, headers: { 'Retry-After': '540' } },
      want: 'Too many tries. Wait 9 minutes and try again.',
    },
    {
      name: 'busy',
      login: { status: 503, body: { error: { code: 'busy', message: 'x' } } },
      want: 'The family hub is busy right now. Try again in a moment.',
    },
    {
      name: 'a server failure without a JSON body',
      login: { status: 502 },
      want: 'Something went wrong. Try again in a moment.',
    },
    {
      name: 'no answer',
      login: 'network error' as const,
      want: 'Can’t reach the family hub. Check your connection and try again.',
    },
  ])('explains a refusal: $name', async ({ login, want }) => {
    stubApi({ providers: noGoogle, login })
    renderLogin()

    signIn()

    expect(await screen.findByRole('alert')).toHaveTextContent(want)
  })

  it.each([
    { name: 'a failed Google sign-in', google: true, want: 'Signing in with Google didn’t work. Sign in with your email' },
    { name: 'Google being off', google: false, want: 'Signing in with Google isn’t available.' },
  ])('explains $name', async ({ google, want }) => {
    stubApi({ providers: { status: 200, body: { google } } })
    const { providersSettled } = renderLogin('/auth/login?google=unavailable')

    await providersSettled()

    expect(screen.getByRole('alert')).toHaveTextContent(want)
  })
})
