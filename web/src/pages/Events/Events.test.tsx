import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { eventKeys, type FamilyEvent } from '../../api/events'
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

function renderPage(events?: FamilyEvent[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  if (events) {
    client.setQueryData(eventKeys.all, events)
  }
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={createMemoryRouter([{ path: '/', element: <Events /> }])} />
    </QueryClientProvider>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Events', () => {
  it('lists each event with its year, date, place, story and who was there', () => {
    renderPage([
      event({
        id: 'e1',
        title: 'Jean-Robert & Nadège’s wedding',
        date: '1984-11-13',
        date_precision: 'day',
        location: 'Flatbush',
        description: 'Scanned from Nadège’s album.',
        people: [
          { id: 'jr', first_name: 'Jean-Robert', last_name: 'Pluviose' },
          { id: 'n', first_name: 'Nadège', last_name: 'Pluviose' },
        ],
      }),
    ])

    const card = within(screen.getByRole('article'))
    expect(screen.getByText('1984')).toBeInTheDocument()
    expect(card.getByRole('heading', { level: 2, name: 'Jean-Robert & Nadège’s wedding' })).toBeInTheDocument()
    expect(card.getByText('November 13, 1984 · Flatbush')).toBeInTheDocument()
    expect(card.getByText('Scanned from Nadège’s album.')).toBeInTheDocument()
    expect(within(card.getByRole('list', { name: 'Who was there' })).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['Jean-Robert Pluviose', '/people/jr'],
      ['Nadège Pluviose', '/people/n'],
    ])
  })

  it('writes each kind of date as the design does', () => {
    renderPage([
      event({ id: 'a', title: 'A month', date: '2025-07', date_precision: 'month' }),
      event({ id: 'b', title: 'A year', date: '1990', date_precision: 'year' }),
      event({ id: 'c', title: 'About a year', date: '1962', date_precision: 'circa' }),
      event({ id: 'd', title: 'Undated' }),
    ])

    const entries = screen.getAllByRole('listitem').filter((li) => li.querySelector('article'))
    expect(entries.map((li) => [li.firstChild?.textContent, within(li.querySelector('article')!).queryByText(/July|1990|Around/)?.textContent ?? null])).toEqual([
      ['2025', 'July 2025'],
      ['1990', '1990'],
      ['c. 1962', 'Around 1962'],
      ['', null],
    ])
  })

  it('says when there are no events yet', () => {
    renderPage([])

    expect(screen.getByText('No events yet.')).toBeInTheDocument()
  })

  it('says why the timeline could not be read', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
    renderPage()

    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })
})
