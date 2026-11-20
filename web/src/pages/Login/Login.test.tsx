import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { Login } from './Login'

const user = {
  id: '01890000-0000-7000-8000-000000000001',
  email: 'maya.pluviose@example.com',
  role: 'admin',
  person_id: '01890000-0000-7000-8000-000000000002',
}

type Answer = { status: number; body?: unknown; headers?: Record<string, string> } | 'network error'

function stubLogin(answer?: Answer) {
  const fetch = vi.fn(() => {
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

function renderLogin(from?: unknown) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(
    [
      { path: '/auth/login', element: <Login /> },
      { path: '/', element: <p>Home page</p> },
      { path: '/tree', element: <p>Tree page</p> },
    ],
    { initialEntries: [{ pathname: '/auth/login', state: from === undefined ? null : { from } }] },
  )
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return client
}

function signIn() {
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: user.email } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'kafe-ak-pen-patat' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Login', () => {
  it('shows the sign-in form', () => {
    stubLogin()
    renderLogin()

    expect(screen.getByRole('heading', { name: 'Lakay se lakay' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Request access' })).toHaveAttribute('href', '/auth/request-access')
    expect(screen.getByText('Already invited? Use the link in your invitation email.')).toBeInTheDocument()
  })

  it('shows and hides the password', () => {
    stubLogin()
    renderLogin()

    fireEvent.click(screen.getByRole('button', { name: 'Show' }))

    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'text')
    expect(screen.getByRole('button', { name: 'Hide' })).toHaveAttribute('aria-pressed', 'true')
  })

  it.each([
    { name: 'back to the page they were headed for', from: '/tree', lands: 'Tree page' },
    { name: 'home when that was another site', from: '//evil.example.com', lands: 'Home page' },
    { name: 'home when that was another site, written with a backslash', from: '/\\evil.example.com', lands: 'Home page' },
    { name: 'home when it isn’t a path', from: 42, lands: 'Home page' },
  ])('signs in and goes $name', async ({ from, lands }) => {
    stubLogin({ status: 200, body: user })
    renderLogin(from)

    signIn()

    expect(await screen.findByText(lands)).toBeInTheDocument()
  })

  it('signs in and goes home', async () => {
    const fetch = stubLogin({ status: 200, body: user })
    renderLogin()

    signIn()

    expect(await screen.findByText('Home page')).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: user.email, password: 'kafe-ak-pen-patat' }),
    })
  })

  it('drops what was cached for whoever signed in before', async () => {
    stubLogin({ status: 200, body: user })
    const client = renderLogin()
    client.setQueryData(['contacts', 'nadege'], [{ kind: 'phone', value: '+1 718 555 0123' }])

    signIn()

    await screen.findByText('Home page')
    expect(client.getQueryData(['contacts', 'nadege'])).toBeUndefined()
    expect(client.getQueryData(authKeys.me)).toEqual(user)
  })

  it('keeps a pending sign-in when the user types', () => {
    const fetch = stubLogin()
    fetch.mockImplementation(() => new Promise<Response>(() => {}))
    renderLogin()

    signIn()
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'typo@example.com' } })

    expect(screen.getByRole('button', { name: 'Signing in…' })).toHaveAttribute('aria-disabled', 'true')
  })

  it('clears a shown error once the user types', async () => {
    stubLogin({ status: 401, body: { error: { code: 'invalid_credentials', message: 'x' } } })
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
    stubLogin(login)
    renderLogin()

    signIn()

    expect(await screen.findByRole('alert')).toHaveTextContent(want)
  })
})
