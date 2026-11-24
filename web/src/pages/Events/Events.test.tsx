import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { domMax, LazyMotion, MotionGlobalConfig } from 'motion/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys, endSession } from '../../api/auth'
import { eventKeys, type FamilyEvent } from '../../api/events'
import { treeKeys } from '../../api/tree'
import { Toaster } from '../../components/Toaster/Toaster'
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

let lastClient: QueryClient

// motion: with Motion's features and animations on, so a closing panel stays mounted while it leaves.
function renderPage(events?: FamilyEvent[], role = 'member', { motion = false } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  lastClient = client
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role, person_id: null })
  client.setQueryData(treeKeys.all, tree)
  if (events) {
    client.setQueryData(eventKeys.all, events)
  }
  const page = (
    <QueryClientProvider client={client}>
      <Toaster>
        <RouterProvider router={createMemoryRouter([{ path: '/', element: <Events /> }])} />
      </Toaster>
    </QueryClientProvider>
  )
  if (motion) {
    MotionGlobalConfig.skipAnimations = false
  }
  render(motion ? <LazyMotion features={domMax}>{page}</LazyMotion> : page)
}

afterEach(() => {
  MotionGlobalConfig.skipAnimations = true
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

  describe('removing an event', () => {
    const wedding = event({ id: 'w', title: 'Wedding', date: '1984', date_precision: 'year' })
    const picnic = event({ id: 'p', title: 'Picnic', date: '2025', date_precision: 'year' })

    function serveEvents(deleteStatus = 204) {
      const deleted: string[] = []
      const fetch = vi.fn((url: string, init?: RequestInit) => {
        if (init?.method === 'DELETE') {
          deleted.push(url)
          return Promise.resolve(deleteStatus === 204 ? new Response(null, { status: 204 }) : Response.json({ error: { code: 'x', message: 'x' } }, { status: deleteStatus }))
        }
        const gone = deleteStatus === 204 && deleted.length > 0
        return Promise.resolve(Response.json({ events: gone ? [picnic] : [picnic, wedding] }))
      })
      vi.stubGlobal('fetch', fetch)
      return deleted
    }
    const titles = () => screen.queryAllByRole('heading', { level: 2 }).map((h) => h.textContent)
    const removeWedding = () => {
      fireEvent.click(screen.getByRole('button', { name: 'Remove “Wedding”' }))
      fireEvent.click(within(screen.getByRole('dialog', { name: 'Remove “Wedding”?' })).getByRole('button', { name: 'Remove' }))
    }
    // Radix closes a toast on Escape, as it would when its 5 seconds run out.
    const letToastRunOut = () => fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })

    it('shows the × to admins only', () => {
      renderPage([picnic, wedding], 'member')

      expect(screen.queryByRole('button', { name: 'Remove “Wedding”' })).toBeNull()
    })

    it('takes it off the timeline at once and keeps it on the server while Undo is offered', async () => {
      const deleted = serveEvents()
      renderPage([picnic, wedding], 'admin')

      removeWedding()

      await waitFor(() => expect(titles()).toEqual(['Picnic']))
      expect(screen.getByText('Removed “Wedding”.')).toBeInTheDocument()
      await act(() => lastClient.refetchQueries({ queryKey: eventKeys.all }))
      expect(titles()).toEqual(['Picnic'])
      fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
      expect(await screen.findByRole('heading', { level: 2, name: 'Wedding' })).toBeInTheDocument()
      expect(deleted).toEqual([])
    })

    it('removes it on the server once Undo has run out', async () => {
      const deleted = serveEvents()
      renderPage([picnic, wedding], 'admin')
      removeWedding()
      await screen.findByText('Removed “Wedding”.')

      letToastRunOut()

      await waitFor(() => expect(deleted).toEqual(['/api/events/w']))
      expect(titles()).toEqual(['Picnic'])
    })

    it('clears the note and sends nothing when the session ends during Undo', async () => {
      const deleted = serveEvents()
      renderPage([picnic, wedding], 'admin')
      removeWedding()
      await screen.findByText('Removed “Wedding”.')

      act(() => endSession(new QueryClient()))

      await waitFor(() => expect(screen.queryByText('Removed “Wedding”.')).toBeNull())
      expect(deleted).toEqual([])
    })

    it('brings it back and says why when the server won’t remove it', async () => {
      serveEvents(503)
      renderPage([picnic, wedding], 'admin')
      removeWedding()
      await screen.findByText('Removed “Wedding”.')

      letToastRunOut()

      expect(await screen.findByText(/^Couldn’t remove “Wedding”\./)).toBeInTheDocument()
      expect(await screen.findByRole('heading', { level: 2, name: 'Wedding' })).toBeInTheDocument()
    })
  })

  describe('the year handle', () => {
    const spread = [
      event({ id: 'a', title: 'Thanksgiving', date: '2026-11-26', date_precision: 'day' }),
      event({ id: 'b', title: 'Graduation', date: '2025-05-30', date_precision: 'day' }),
      event({ id: 'c', title: 'Reunion', date: '2025-07', date_precision: 'month' }),
      event({ id: 'd', title: 'Wedding', date: '1984', date_precision: 'year' }),
      event({ id: 'e', title: 'Picnic' }),
    ]
    const handle = () => within(screen.getByRole('group', { name: 'Move through the years' }))
    const lit = () => [...document.querySelectorAll('[class*="yearLit"]')].map((el) => el.textContent)

    it.each([
      { name: 'the ▼ button', go: () => fireEvent.click(handle().getByRole('button', { name: 'Older year' })) },
      { name: 'the down arrow key', go: () => fireEvent.keyDown(handle().getByRole('slider', { name: 'Year' }), { key: 'ArrowDown' }) },
    ])('steps to the next older year with $name, lighting every event of that year', ({ go }) => {
      const scrollTo = vi.fn()
      vi.stubGlobal('scrollTo', scrollTo)
      renderPage(spread)
      expect(handle().getByRole('button', { name: 'Newer year' })).toBeDisabled()
      expect(lit()).toEqual(['2026'])

      go()

      expect(scrollTo).toHaveBeenCalledTimes(1)
      expect(handle().getByRole('slider')).toHaveAttribute('aria-valuetext', '2025')
      expect(lit()).toEqual(['2025', '2025'])
    })

    it('scrubs a year for every 26px dragged, and reaches the undated events last', () => {
      vi.stubGlobal('scrollTo', vi.fn())
      renderPage(spread)
      const grip = handle().getByRole('slider')

      fireEvent.pointerDown(grip, { button: 0, pointerId: 1, clientY: 100 })
      expect(grip).toHaveFocus()
      fireEvent.pointerMove(grip, { pointerId: 1, clientY: 152 })
      expect(grip).toHaveAttribute('aria-valuetext', '1984')
      fireEvent.pointerMove(grip, { pointerId: 1, clientY: 400 })
      fireEvent.pointerUp(grip, { pointerId: 1 })

      expect(grip).toHaveAttribute('aria-valuetext', 'Undated')
      expect(handle().getByRole('button', { name: 'Older year' })).toBeDisabled()
    })

    it('goes to the oldest with End and the newest with Home', () => {
      vi.stubGlobal('scrollTo', vi.fn())
      renderPage(spread)
      const grip = handle().getByRole('slider')

      fireEvent.keyDown(grip, { key: 'End' })
      expect(grip).toHaveAttribute('aria-valuetext', 'Undated')
      fireEvent.keyDown(grip, { key: 'Home' })
      expect(grip).toHaveAttribute('aria-valuetext', '2026')
    })

    it('keeps a picked year through the page’s scrolling until the reader scrolls', () => {
      vi.stubGlobal('scrollTo', vi.fn())
      renderPage(spread)
      fireEvent.click(handle().getByRole('button', { name: 'Older year' }))

      // jsdom lays nothing out, so following the scroll would land on the last year.
      fireEvent.scroll(window)
      expect(handle().getByRole('slider')).toHaveAttribute('aria-valuetext', '2025')

      fireEvent.wheel(window)
      fireEvent.scroll(window)
      expect(handle().getByRole('slider')).toHaveAttribute('aria-valuetext', 'Undated')
    })

    it('changes the year without its fade with reduced motion', () => {
      vi.stubGlobal('scrollTo', vi.fn())
      vi.stubGlobal('matchMedia', (query: string) => ({
        matches: query.includes('reduce'),
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {},
      }))
      renderPage(spread, 'member', { motion: true })

      fireEvent.click(handle().getByRole('button', { name: 'Older year' }))

      // A fading year would still be there, leaving.
      expect(handle().getByRole('slider').textContent).toBe('’25')
    })

    it('isn’t there when every event falls in one year', () => {
      renderPage([spread[1]!, spread[2]!])

      expect(screen.queryByRole('group', { name: 'Move through the years' })).toBeNull()
      expect(lit()).toEqual([])
    })
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

  it('waits for a half-typed date to be finished or cleared', () => {
    serve()
    renderPage([], 'admin')
    open()
    type('What happened?', 'Wedding')
    const date = panel().getByLabelText('Date')
    // A date picker holding part of a date reports an empty value and badInput.
    const halfTyped = (bad: boolean) => {
      Object.defineProperty(date, 'validity', { configurable: true, value: { badInput: bad } })
      fireEvent.input(date, { target: { value: '' } })
    }

    halfTyped(true)
    expect(panel().getByRole('button', { name: 'Save event' })).toBeDisabled()
    expect(panel().getByText('Finish the date, or clear it.')).toBeInTheDocument()

    halfTyped(false)
    expect(panel().getByRole('button', { name: 'Save event' })).toBeEnabled()
  })

  it('forgets a half-typed date once another precision is picked', () => {
    serve()
    renderPage([], 'admin')
    open()
    type('What happened?', 'Wedding')
    const date = panel().getByLabelText('Date')
    Object.defineProperty(date, 'validity', { configurable: true, value: { badInput: true } })
    fireEvent.input(date, { target: { value: '' } })
    fireEvent.click(panel().getByRole('button', { name: 'Exact day' }))
    expect(panel().getByRole('button', { name: 'Save event' })).toBeDisabled()

    fireEvent.click(panel().getByRole('button', { name: 'Year' }))
    fireEvent.click(panel().getByRole('button', { name: 'Exact day' }))

    expect(panel().getByRole('button', { name: 'Save event' })).toBeEnabled()
    expect(panel().queryByText('Finish the date, or clear it.')).toBeNull()
  })

  it('keeps what was typed and says why when saving fails', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ error: { code: 'validation_error', message: 'invalid event: someone tagged isn’t in the tree' } }, { status: 400 }))))
    renderPage([], 'admin')
    open()
    type('What happened?', 'Wedding')
    fireEvent.click(panel().getByRole('button', { name: 'Save event' }))

    expect(await panel().findByRole('alert')).toHaveTextContent('Someone tagged isn’t in the tree.')
    expect(panel().getByLabelText('What happened?')).toHaveValue('Wedding')
  })

  it('locks the panel and + New event while saving', async () => {
    let answer: (r: Response) => void = () => {}
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string, init?: RequestInit) =>
        init?.method === 'POST'
          ? new Promise<Response>((resolve) => (answer = resolve))
          : Promise.resolve(Response.json(url === '/api/events' ? { events: [] } : { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: null })),
      ),
    )
    renderPage([], 'admin')
    open()
    type('What happened?', 'Wedding')
    fireEvent.click(panel().getByRole('button', { name: 'Save event' }))

    await vi.waitFor(() => expect(panel().getByLabelText('What happened?')).toBeDisabled())
    expect(panel().getByRole('button', { name: 'Cancel' })).toBeDisabled()
    expect(panel().getByRole('button', { name: 'Close' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '+ New event' })).toBeDisabled()

    answer(Response.json(event({ id: 'new', title: 'Wedding' }), { status: 201 }))
    expect(await screen.findByText('Saved “Wedding”.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '+ New event' })).toBeEnabled()
  })

  it('keeps focus in the picker when a person is removed', async () => {
    serve()
    renderPage([], 'admin')
    open()
    fireEvent.click(panel().getByRole('button', { name: '+ Add person' }))
    type('Find a relative', 'rose')
    fireEvent.click(panel().getByRole('button', { name: 'Rose Monestime' }))

    // The pick hands focus back to + Add person on the next frame; the × is focused after that, as a keyboard would.
    await vi.waitFor(() => expect(panel().getByRole('button', { name: '+ Add person' })).toHaveFocus())
    const remove = panel().getByRole('button', { name: 'Remove Rose Monestime' })
    remove.focus()
    fireEvent.click(remove)

    await vi.waitFor(() => expect(panel().getByRole('button', { name: '+ Add person' })).toHaveFocus())
  })

  it('still says no one matched when a reread of the tree fails', async () => {
    serve()
    renderPage([], 'admin')
    open()
    fireEvent.click(panel().getByRole('button', { name: '+ Add person' }))
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
    await act(() => lastClient.refetchQueries({ queryKey: treeKeys.all }))
    type('Find a relative', 'zzz')

    expect(panel().getByText('No one by that name.')).toBeInTheDocument()
  })

  it('says the search failed when the tree can’t be read', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => (url === '/api/tree' ? Promise.reject(new TypeError('Failed to fetch')) : Promise.resolve(Response.json({ events: [] })))))
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: null })
    client.setQueryData(eventKeys.all, [])
    render(
      <QueryClientProvider client={client}>
        <Toaster>
        <RouterProvider router={createMemoryRouter([{ path: '/', element: <Events /> }])} />
      </Toaster>
      </QueryClientProvider>,
    )
    open()
    fireEvent.click(panel().getByRole('button', { name: '+ Add person' }))
    type('Find a relative', 'rose')

    expect(await panel().findByRole('alert')).toHaveTextContent('Can’t search the family tree right now.')
    expect(panel().queryByText('No one by that name.')).toBeNull()
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
