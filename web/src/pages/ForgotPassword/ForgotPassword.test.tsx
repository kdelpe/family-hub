import { onlineManager, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { createQueryClient } from '../../api/queryClient'
import { ForgotPassword } from './ForgotPassword'

const email = 'maya.pluviose@example.com'

function renderPage(answer: Response | Promise<Response> | 'network error') {
  const fetch = vi.fn(() =>
    answer === 'network error' ? Promise.reject(new TypeError('Failed to fetch')) : Promise.resolve(answer),
  )
  vi.stubGlobal('fetch', fetch)
  const router = createMemoryRouter([{ path: '/auth/forgot', element: <ForgotPassword /> }], {
    initialEntries: ['/auth/forgot'],
  })
  render(
    <QueryClientProvider client={createQueryClient()}>
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
  onlineManager.setOnline(true)
})

describe('ForgotPassword', () => {
  it('sends the email and says to check it', async () => {
    const fetch = renderPage(new Response(null, { status: 202 }))

    send()

    expect(await screen.findByRole('heading', { name: 'Check your email' })).toHaveFocus()
    expect(screen.getByText(/It works for 30 minutes\./)).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith('/api/auth/password/forgot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    })
  })

  it('fails instead of waiting when the browser says it is offline', async () => {
    renderPage('network error')
    onlineManager.setOnline(false)

    send()

    expect(await screen.findByRole('alert')).toHaveTextContent('Can’t reach the family hub.')
    expect(screen.getByLabelText('Email')).not.toHaveAttribute('readonly')
  })

  it('sends once and locks the email while sending', async () => {
    const fetch = renderPage(new Promise<Response>(() => {}))

    send()
    fireEvent.click(await screen.findByRole('button', { name: 'Sending…' }))
    // A second send would reach fetch a tick later, so give it one.
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(fetch).toHaveBeenCalledTimes(1)
    expect(screen.getByLabelText('Email')).toHaveAttribute('readonly')
  })

  it.each([
    {
      name: 'too many tries',
      answer: Response.json({ error: { code: 'rate_limited', message: 'x' } }, { status: 429, headers: { 'Retry-After': '3600' } }),
      want: 'Too many tries. Wait 60 minutes and try again.',
    },
    {
      name: 'busy',
      answer: Response.json({ error: { code: 'busy', message: 'x' } }, { status: 503 }),
      want: 'The family hub is busy right now.',
    },
    { name: 'a server failure', answer: new Response(null, { status: 500 }), want: 'Something went wrong.' },
    { name: 'no answer', answer: 'network error' as const, want: 'Can’t reach the family hub.' },
  ])('explains a failure: $name', async ({ answer, want }) => {
    renderPage(answer)

    send()

    expect(await screen.findByRole('alert')).toHaveTextContent(want)
    expect(screen.getByRole('button', { name: 'Send me a reset link' })).not.toHaveAttribute('aria-disabled', 'true')
  })
})
