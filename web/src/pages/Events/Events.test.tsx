import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { eventKeys, type FamilyEvent } from '../../api/events'
import { treeKeys } from '../../api/tree'
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

const person = (id: string, first_name: string, last_name: string) => ({ id, first_name, last_name, side: 'first', gender: 'female', is_deceased: false })
const tree = { persons: [person('rose', 'Rose', 'Monestime'), person('fritz', 'Fritz', 'Pluviose'), person('frantz', 'Frantz', 'Pluviose')], relationships: [] }

function renderPage(events?: FamilyEvent[], role = 'member') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role, person_id: null })
  client.setQueryData(treeKeys.all, tree)
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

  it('offers + New event to admins only', () => {
    renderPage([])
    expect(screen.queryByRole('button', { name: '+ New event' })).toBeNull()
  })
})

describe('New event', () => {
  // The server: saves what it's sent, then lists it; it also answers who's signed in and the tree.
  function serve() {
    const sent: unknown[] = []
    const fetch = vi.fn((url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        const body = JSON.parse(String(init.body)) as { title: string }
        sent.push(body)
        return Promise.resolve(Response.json(event({ id: 'new', title: body.title }), { status: 201 }))
      }
      if (url === '/api/auth/me') {
        return Promise.resolve(Response.json({ id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: null }))
      }
      if (url === '/api/tree') {
        return Promise.resolve(Response.json(tree))
      }
      return Promise.resolve(Response.json({ events: sent.map((b, i) => event({ id: `e${i}`, title: (b as { title: string }).title })) }))
    })
    vi.stubGlobal('fetch', fetch)
    return sent
  }

  const open = () => fireEvent.click(screen.getByRole('button', { name: '+ New event' }))
  const panel = () => within(screen.getByRole('region', { name: 'New event' }))
  const type = (label: string, value: string) => fireEvent.change(panel().getByLabelText(label), { target: { value } })

  it('saves what was typed, the date at its precision and who was there', async () => {
    const sent = serve()
    renderPage([], 'admin')
    open()
    expect(panel().getByLabelText('What happened?')).toHaveFocus()

    type('What happened?', '  Rose’s kitchen ')
    type('Where?', 'Jacmel')
    fireEvent.click(panel().getByRole('button', { name: 'About' }))
    type('About which year', '1962')
    type('Tell the story (optional)', 'The earliest photos we have.')
    fireEvent.click(panel().getByRole('button', { name: '+ Add person' }))
    type('Find a relative', 'ros')
    fireEvent.click(panel().getByRole('button', { name: 'Rose Monestime' }))
    fireEvent.click(panel().getByRole('button', { name: '+ Add person' }))
    type('Find a relative', 'pluv')
    expect(panel().getAllByRole('button', { name: /Pluviose/ }).map((b) => b.textContent)).toEqual(['Fritz Pluviose', 'Frantz Pluviose'])
    fireEvent.click(panel().getByRole('button', { name: 'Fritz Pluviose' }))
    fireEvent.click(panel().getByRole('button', { name: 'Remove Rose Monestime' }))
    fireEvent.click(panel().getByRole('button', { name: 'Save event' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Saved “Rose’s kitchen”.')
    expect(sent).toEqual([
      { title: 'Rose’s kitchen', date: '1962', date_precision: 'circa', location: 'Jacmel', description: 'The earliest photos we have.', person_ids: ['fritz'] },
    ])
    expect(screen.queryByRole('region', { name: 'New event' })).toBeNull()
    expect(await screen.findByRole('heading', { level: 2, name: 'Rose’s kitchen' })).toBeInTheDocument()
  })

  it('sends a month as YYYY-MM and an exact day as picked', async () => {
    const sent = serve()
    renderPage([], 'admin')
    open()
    type('What happened?', 'Naomie is born')
    type('Date', '2025-07-02')
    fireEvent.click(panel().getByRole('button', { name: 'Save event' }))
    await screen.findByText('Saved “Naomie is born”.')

    open()
    type('What happened?', 'Thanksgiving')
    fireEvent.click(panel().getByRole('button', { name: 'Month' }))
    fireEvent.change(panel().getByLabelText('Month', { selector: 'select' }), { target: { value: '11' } })
    type('Year', '2026')
    fireEvent.click(panel().getByRole('button', { name: 'Save event' }))
    await screen.findByText('Saved “Thanksgiving”.')

    expect(sent).toEqual([
      { title: 'Naomie is born', date: '2025-07-02', date_precision: 'day', person_ids: [] },
      { title: 'Thanksgiving', date: '2026-11', date_precision: 'month', person_ids: [] },
    ])
  })

  it('waits for a title and a whole year before saving', () => {
    serve()
    renderPage([], 'admin')
    open()
    const save = panel().getByRole('button', { name: 'Save event' })
    expect(save).toBeDisabled()

    type('What happened?', 'Wedding')
    expect(save).toBeEnabled()
    fireEvent.click(panel().getByRole('button', { name: 'Year' }))
    type('Year', '196')
    expect(save).toBeDisabled()
    expect(panel().getByText('Write the year with four digits, like 1965.')).toBeInTheDocument()

    fireEvent.click(panel().getByRole('button', { name: 'Month' }))
    type('Year', '1962')
    expect(save).toBeDisabled()
    expect(panel().getByText('Pick the month too.')).toBeInTheDocument()
  })

  it('keeps what was typed and says why when saving fails', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ error: { code: 'validation_error', message: 'invalid event: someone tagged isn’t in the tree' } }, { status: 400 }))))
    renderPage([], 'admin')
    open()
    type('What happened?', 'Wedding')
    fireEvent.click(panel().getByRole('button', { name: 'Save event' }))

    expect(await panel().findByRole('alert')).toBeInTheDocument()
    expect(panel().getByLabelText('What happened?')).toHaveValue('Wedding')
  })

  it('closes from Cancel and ×, handing focus back to + New event', async () => {
    serve()
    renderPage([], 'admin')
    for (const name of ['Cancel', 'Close']) {
      open()
      fireEvent.click(panel().getByRole('button', { name }))

      expect(screen.queryByRole('region', { name: 'New event' })).toBeNull()
      await vi.waitFor(() => expect(screen.getByRole('button', { name: '+ New event' })).toHaveFocus())
    }
  })
})
