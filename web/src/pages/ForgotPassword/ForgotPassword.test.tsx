import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ForgotPassword } from './ForgotPassword'

const email = 'maya.pluviose@example.com'

function renderPage(answer: Response | 'network error') {
  const fetch = vi.fn(() => (answer === 'network error' ? Promise.reject(new TypeError('Failed to fetch')) : Promise.resolve(answer)))
  vi.stubGlobal('fetch', fetch)
  const router = createMemoryRouter([{ path: '/auth/forgot', element: <ForgotPassword /> }], {
    initialEntries: ['/auth/forgot'],
  })
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return fetch
}

function send() {
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: email } })
  fireEvent.click(screen.getByRole('button', { name: 'Send me a reset link' }))
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ForgotPassword', () => {
  it('sends the email and says to check it', async () => {
    const fetch = renderPage(new Response(null, { status: 202 }))

    send()

    expect(await screen.findByRole('heading', { name: 'Check your email' })).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('It works for 30 minutes.')
    expect(fetch).toHaveBeenCalledWith('/api/auth/password/forgot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
      signal: undefined,
    })
  })

  it.each([
    {
      name: 'too many tries',
      answer: Response.json({ error: { code: 'rate_limited', message: 'x' } }, { status: 429, headers: { 'Retry-After': '3600' } }),
      want: 'Too many tries. Wait 60 minutes and try again.',
    },
    { name: 'no answer', answer: 'network error' as const, want: 'Can’t reach the family hub.' },
  ])('explains a failure: $name', async ({ answer, want }) => {
    renderPage(answer)

    send()

    expect(await screen.findByRole('alert')).toHaveTextContent(want)
    expect(screen.getByRole('button', { name: 'Send me a reset link' })).toBeEnabled()
  })
})
