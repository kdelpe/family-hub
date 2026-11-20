import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { SuggestCard } from './SuggestCard'

type Answer = { status: number; body?: unknown }

function renderCard(answers: Record<string, Answer> = {}) {
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const answer = answers[`${init?.method ?? 'GET'} ${url}`]
    return answer
      ? Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
      : Promise.reject(new TypeError('Failed to fetch'))
  })
  vi.stubGlobal('fetch', fetch)
  render(
    <QueryClientProvider client={new QueryClient()}>
      <SuggestCard yourName="Maya Pluviose" />
    </QueryClientProvider>,
  )
  return { fetch }
}

const form = () => within(screen.getByRole('form', { name: 'Suggest a relative' }))

const fill = (fields: Record<string, string>) => {
  for (const [label, value] of Object.entries(fields)) {
    fireEvent.change(form().getByLabelText(label), { target: { value } })
  }
}

const suggested = (fetch: ReturnType<typeof renderCard>['fetch']) =>
  fetch.mock.calls.find(([url, init]) => url === '/api/me/suggestions' && init?.method === 'POST')?.[1]?.body

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('SuggestCard', () => {
  it.each<{ name: string; fields: Record<string, string>; body: object }>([
    {
      name: 'everything filled in, related to you by default',
      fields: { 'Their full name': ' Darline Monestime ', 'Their email': 'darline@example.com', Relationship: 'cousin', 'Anything else? (optional)': 'Lives in Montréal.' },
      body: { full_name: 'Darline Monestime', email: 'darline@example.com', relationship: 'cousin', related_names: ['Maya Pluviose'], note: 'Lives in Montréal.' },
    },
    {
      name: 'only a name and an email',
      fields: { 'Their full name': 'Darline Monestime', 'Their email': 'darline@example.com', 'Related to': ' ' },
      body: { full_name: 'Darline Monestime', email: 'darline@example.com', relationship: null, related_names: [], note: '' },
    },
  ])('sends a suggestion with $name, then thanks you', async ({ fields, body }) => {
    const { fetch } = renderCard({ 'POST /api/me/suggestions': { status: 202, body: {} } })
    expect(
      screen.getByText('Suggest a relative and an admin will take a look. If they approve, your relative gets an invitation by email.'),
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Suggest a relative' }))
    expect(form().getByLabelText('Related to')).toHaveValue('Maya Pluviose')
    fill(fields)
    fireEvent.click(form().getByRole('button', { name: 'Send suggestion' }))

    expect(
      await screen.findByText('Thanks. An admin will take a look, and if they approve, Darline gets an invitation by email.'),
    ).toBeInTheDocument()
    expect(suggested(fetch)).toBe(JSON.stringify(body))
    expect(screen.queryByRole('form')).not.toBeInTheDocument()
  })

  it('opens a fresh form for someone else', async () => {
    renderCard({ 'POST /api/me/suggestions': { status: 202, body: {} } })
    fireEvent.click(screen.getByRole('button', { name: 'Suggest a relative' }))
    fill({ 'Their full name': 'Darline Monestime', 'Their email': 'darline@example.com' })
    fireEvent.click(form().getByRole('button', { name: 'Send suggestion' }))

    fireEvent.click(await screen.findByRole('button', { name: 'Suggest someone else' }))

    expect(form().getByLabelText('Their full name')).toHaveValue('')
    expect(screen.queryByText(/Thanks/)).not.toBeInTheDocument()
  })

  it.each([
    {
      name: 'a blank name is caught before sending',
      fields: { 'Their full name': '  ', 'Their email': 'darline@example.com' },
      answer: undefined,
      says: 'Enter their full name.',
    },
    {
      name: 'the server’s refusal is shown in its own words',
      fields: { 'Their full name': 'Darline Monestime', 'Their email': 'darline@' },
      answer: {
        status: 400,
        body: { error: { code: 'validation_error', message: 'invalid access request: the email must be an address like name@example.com' } },
      },
      says: 'The email must be an address like name@example.com.',
    },
  ])('$name', async ({ fields, answer, says }) => {
    const { fetch } = renderCard(answer ? { 'POST /api/me/suggestions': answer } : {})
    fireEvent.click(screen.getByRole('button', { name: 'Suggest a relative' }))
    fill(fields)

    fireEvent.click(form().getByRole('button', { name: 'Send suggestion' }))

    expect(await form().findByRole('alert')).toHaveTextContent(says)
    expect(suggested(fetch) !== undefined).toBe(answer !== undefined)
  })

  it('cancels without sending', () => {
    const { fetch } = renderCard()
    fireEvent.click(screen.getByRole('button', { name: 'Suggest a relative' }))
    fill({ 'Their full name': 'Darline Monestime' })

    fireEvent.click(form().getByRole('button', { name: 'Cancel' }))

    expect(screen.queryByRole('form')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Suggest a relative' })).toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalled()
  })
})
