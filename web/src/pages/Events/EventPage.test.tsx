import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it } from 'vitest'

import { authKeys } from '../../api/auth'
import { eventKeys, type FamilyEvent } from '../../api/events'
import { EventPage } from './EventPage'
import { hideEvent, unhideEvent } from './pendingRemovals'
import { Events } from './Events'

const event = (over: Partial<FamilyEvent>): FamilyEvent => ({
  id: 'e1',
  title: 'Wedding',
  date: null,
  date_precision: null,
  location: null,
  description: null,
  people: [],
  created_at: '2026-11-21T10:00:00Z',
  ...over,
})

// Newest first, as the server sends them.
const events = [
  event({ id: 'thanks', title: 'Thanksgiving at Fritz’s', date: '2026-11', date_precision: 'month', location: 'Brooklyn', description: 'Everyone under one roof.' }),
  event({ id: 'naomie', title: 'Naomie is born', date: '2025-07-02', date_precision: 'day' }),
  event({ id: 'wedding', title: 'Jean-Robert & Nadège’s wedding', date: '1984', date_precision: 'year' }),
]

function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'member', person_id: null })
  client.setQueryData(eventKeys.all, events)
  const router = createMemoryRouter(
    [
      { path: '/events', element: <Events /> },
      { path: '/events/:id', element: <EventPage /> },
    ],
    { initialEntries: [path] },
  )
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return router
}

afterEach(() => {
  unhideEvent('naomie')
  unhideEvent('wedding')
})

describe('EventPage', () => {
  it('shows the event, its story and a cover waiting for its first photo', () => {
    renderAt('/events/thanks')

    expect(screen.getByRole('heading', { level: 1, name: 'Thanksgiving at Fritz’s' })).toBeInTheDocument()
    expect(screen.getByText('November 2026 · Brooklyn')).toBeInTheDocument()
    expect(screen.getByText('Everyone under one roof.')).toBeInTheDocument()
    expect(screen.getByText('No photos or videos yet')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '‹ Events' })).toHaveAttribute('href', '/events')
  })

  it.each([
    { name: 'the newest, with only an older one', id: 'thanks', newer: null, older: 'Naomie is born · 2025' },
    { name: 'one in the middle, with both', id: 'naomie', newer: 'Thanksgiving at Fritz’s · 2026', older: 'Jean-Robert & Nadège’s wedding · 1984' },
    { name: 'the oldest, with only a newer one', id: 'wedding', newer: 'Naomie is born · 2025', older: null },
  ])('links to the events either side: $name', ({ id, newer, older }) => {
    renderAt(`/events/${id}`)

    const link = (label: string, title: string | null) =>
      title === null ? expect(screen.queryByRole('link', { name: new RegExp(`^${label}`) })).toBeNull() : expect(screen.getByRole('link', { name: `${label}${title}` })).toBeInTheDocument()
    link('‹ Newer', newer)
    link('Older ›', older)
  })

  it.each([
    { name: 'one with another beyond it', from: 'thanks', link: 'Older ›Naomie is born · 2025', to: 'Naomie is born' },
    { name: 'the newest, whose Newer link goes', from: 'naomie', link: '‹ NewerThanksgiving at Fritz’s · 2026', to: 'Thanksgiving at Fritz’s' },
    { name: 'the oldest, whose Older link goes', from: 'naomie', link: 'Older ›Jean-Robert & Nadège’s wedding · 1984', to: 'Jean-Robert & Nadège’s wedding' },
  ])('moves focus to the top of the next event: $name', ({ from, link, to }) => {
    renderAt(`/events/${from}`)
    const followed = screen.getByRole('link', { name: link })
    followed.focus()

    fireEvent.click(followed)

    expect(screen.getByRole('heading', { level: 1, name: to })).toBeInTheDocument()
    expect(screen.getByRole('main')).toHaveFocus()
  })

  it('says so when the event isn’t there', () => {
    renderAt('/events/gone')

    expect(screen.getByText('This event isn’t in the album. It may have been removed.')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { level: 1 })).toBeNull()
  })

  it('skips an event being removed, while its Undo still shows', () => {
    hideEvent('naomie')
    renderAt('/events/thanks')

    expect(screen.getByRole('link', { name: 'Older ›Jean-Robert & Nadège’s wedding · 1984' })).toBeInTheDocument()
  })

  it('leaves out the other events’ links when there are none', () => {
    hideEvent('naomie')
    hideEvent('wedding')
    renderAt('/events/thanks')

    expect(screen.queryByRole('navigation', { name: 'Other events' })).toBeNull()
  })

  it('opens from its card on the timeline', () => {
    const router = renderAt('/events')

    fireEvent.click(screen.getByRole('link', { name: 'Naomie is born' }))

    expect(router.state.location.pathname).toBe('/events/naomie')
    expect(screen.getByRole('heading', { level: 1, name: 'Naomie is born' })).toBeInTheDocument()
  })
})
