import { QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { createQueryClient } from '../../api/queryClient'
import { RequestAccess } from './RequestAccess'

function renderPage(answer: () => Promise<Response> = () => Promise.resolve(new Response(null, { status: 202 }))) {
  const fetch = vi.fn(answer)
  vi.stubGlobal('fetch', fetch)
  const router = createMemoryRouter([{ path: '/auth/request-access', element: <RequestAccess /> }], {
    initialEntries: ['/auth/request-access'],
  })
  render(
    <QueryClientProvider client={createQueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return fetch
}

function type(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } })
}

function fillRequired() {
  type('Your full name', '  Darline Monestime ')
  type('Email', 'darline.monestime@example.com')
}

const send = () => fireEvent.click(screen.getByRole('button', { name: 'Send my request' }))

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('RequestAccess', () => {
  it('sends the request and thanks the person by first name', async () => {
    const fetch = renderPage()
    fillRequired()
    fireEvent.click(screen.getByRole('radio', { name: 'Pluviose family' }))
    fireEvent.change(screen.getByLabelText('Relationship'), { target: { value: 'son' } })
    type('Their full name', ' Frantz Monestime ')
    type('Anything else? (optional)', 'Cousin Maya told me about the site.')

    send()

    expect(await screen.findByRole('heading', { name: 'Thank you, Darline' })).toHaveFocus()
    expect(fetch).toHaveBeenCalledWith('/api/access-requests', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        full_name: 'Darline Monestime',
        email: 'darline.monestime@example.com',
        side: 'first',
        relationship: 'son',
        related_names: ['Frantz Monestime'],
        note: 'Cousin Maya told me about the site.',
        website: '',
      }),
    })
  })

  it('sends what a bot puts in the hidden website field, so the server can drop it', async () => {
    const fetch = renderPage()
    fillRequired()
    const trap = document.getElementById('ra-website')!
    expect(trap.closest('[aria-hidden="true"]')).not.toBeNull()
    expect(trap).toHaveAttribute('tabindex', '-1')

    fireEvent.change(trap, { target: { value: 'https://spam.example.com' } })
    send()

    await screen.findByRole('heading', { name: 'Thank you, Darline' })
    expect(fetch).toHaveBeenCalledWith(
      '/api/access-requests',
      expect.objectContaining({ body: expect.stringContaining('"website":"https://spam.example.com"') }),
    )
  })

  it('offers the family or married in as one choice', () => {
    renderPage()
    const pluviose = screen.getByRole('radio', { name: 'Pluviose family' })
    const married = screen.getByRole('radio', { name: 'Married into the family' })

    fireEvent.click(pluviose)
    expect(pluviose).toBeChecked()

    fireEvent.click(married)
    expect(pluviose).not.toBeChecked()
    expect(married).toBeChecked()
  })

  it('adds name fields up to four, focusing each new one', () => {
    renderPage()
    expect(screen.getAllByPlaceholderText('Their full name')).toHaveLength(2)

    fireEvent.click(screen.getByRole('button', { name: '+ Add another name' }))
    fireEvent.click(screen.getByRole('button', { name: '+ Add another name' }))

    expect(screen.getAllByPlaceholderText('Their full name')).toHaveLength(4)
    expect(screen.getAllByPlaceholderText('Their full name')[3]).toHaveFocus()
    expect(screen.queryByRole('button', { name: '+ Add another name' })).not.toBeInTheDocument()
  })

  it('asks for a name made of more than spaces before sending', () => {
    const fetch = renderPage()
    type('Your full name', '   ')
    type('Email', 'darline.monestime@example.com')

    send()

    expect(screen.getByRole('alert')).toHaveTextContent('Enter your full name.')
    expect(screen.getByLabelText('Your full name')).toHaveFocus()
    expect(fetch).not.toHaveBeenCalled()
  })

  it('shows a failed send and clears the message on the next edit', async () => {
    renderPage(() => Promise.resolve(Response.json({ error: { code: 'rate_limited', message: 'x' } }, { status: 429 })))
    fillRequired()

    send()

    expect(await screen.findByRole('alert')).toHaveTextContent('Too many tries.')
    type('Email', 'darline@example.com')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('sends once and locks the form while sending', async () => {
    const fetch = renderPage(() => new Promise<Response>(() => {}))
    fillRequired()

    send()
    fireEvent.click(await screen.findByRole('button', { name: 'Sending…' }))
    // A second send would reach fetch a tick later, so give it one.
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(fetch).toHaveBeenCalledTimes(1)
    expect(screen.getByLabelText('Email')).toHaveAttribute('readonly')
    fireEvent.click(screen.getByRole('radio', { name: 'Pluviose family' }))
    expect(screen.getByRole('radio', { name: 'Pluviose family' })).not.toBeChecked()
  })
})
