import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, createEvent, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import type { ContactMethod, DirectoryEntry } from '../../api/contacts'
import type { Tree, TreePerson } from '../../api/tree'
import { Members } from './Members'

const person = (id: string, first: string, over: Partial<TreePerson> = {}): TreePerson => ({
  id,
  first_name: first,
  last_name: 'Pluviose',
  side: 'first',
  gender: 'female',
  is_deceased: false,
  bio_excerpt: null,
  ...over,
})

const tree: Tree = {
  persons: [
    person('maya', 'Maya', { side: 'both' }),
    person('fritz', 'Fritz', { side: 'both', gender: 'male' }),
    person('nadege', 'Nadège', { side: 'second' }),
    person('naomie', 'Naomie', { last_name: 'Désir', side: 'married_in' }),
    person('joseph', 'Joseph', { gender: 'male', is_deceased: true, bio_excerpt: 'Drove a yellow cab in New York.' }),
  ],
  relationships: [
    { id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'maya' },
    { id: 'r2', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'fritz' },
    { id: 'r3', type: 'parent_child', subtype: 'biological', parent_id: 'maya', child_id: 'naomie' },
  ],
}

const method = (over: Partial<ContactMethod> & Pick<ContactMethod, 'id' | 'kind' | 'value'>): ContactMethod => ({
  platform: null,
  label: null,
  visibility: 'family',
  href: null,
  shared_with_you: false,
  ...over,
})

const people: DirectoryEntry[] = [
  {
    id: 'fritz',
    first_name: 'Fritz',
    last_name: 'Pluviose',
    side: 'both',
    has_account: true,
    contacts: [
      method({ id: 'c1', kind: 'email', value: 'fritz.p@example.com' }),
      method({ id: 'c2', kind: 'social', platform: 'youtube', value: '@fritzpluviose', href: 'https://www.youtube.com/@fritzpluviose' }),
      method({ id: 'c3', kind: 'phone', value: '(555) 010-2231', visibility: 'on_request', shared_with_you: true }),
      method({ id: 'c4', kind: 'email', value: 'hidden@example.com', visibility: 'hidden' }),
    ],
  },
  { id: 'maya', first_name: 'Maya', last_name: 'Pluviose', side: 'both', has_account: true, contacts: [] },
  {
    id: 'nadege',
    first_name: 'Nadège',
    last_name: 'Pluviose',
    side: 'second',
    has_account: true,
    contacts: [
      { kind: 'address', requestable: true, my_request_status: 'none' },
      { kind: 'phone', requestable: true, my_request_status: 'requested' },
    ],
  },
  { id: 'naomie', first_name: 'Naomie', last_name: 'Désir', side: 'married_in', has_account: false, contacts: [] },
]

// Reduced motion turns pages at once, so the tests needn't wait out the animation. Returns a switch for it.
function stubMedia({ narrow = false, reduced = true } = {}) {
  const state = { reduced }
  const listeners = new Set<() => void>()
  vi.stubGlobal('matchMedia', (query: string) => ({
    get matches() {
      return query.includes('reduce') ? state.reduced : query.includes('max-width') ? narrow : false
    },
    media: query,
    addEventListener: (_: string, listener: () => void) => listeners.add(listener),
    removeEventListener: (_: string, listener: () => void) => listeners.delete(listener),
  }))
  return (on: boolean) => {
    state.reduced = on
    listeners.forEach((listener) => listener())
  }
}

async function renderPage(directory: () => DirectoryEntry[] = () => people) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const body =
        url === '/api/directory'
          ? { people: directory() }
          : url === '/api/tree'
            ? tree
            : init?.method === 'POST' && url === '/api/persons/nadege/contact-requests'
              ? { kind: 'address', my_request_status: 'requested' }
              : null
      return body ? Promise.resolve(new Response(JSON.stringify(body), { status: 200 })) : Promise.reject(new TypeError('Failed to fetch'))
    }),
  )
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u', email: 'maya.pluviose@example.com', role: 'member', person_id: 'maya' })
  const router = createMemoryRouter([{ path: '/members', element: <Members /> }], { initialEntries: ['/members'] })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  await screen.findByRole('group', { name: /^Members book/ })
  return client
}

const book = () => screen.getByRole('group', { name: /^Members book/ })
// The line under the book saying where it's open.
const position = () => document.querySelector('[aria-live]')!.textContent
const press = (name: string | RegExp) => fireEvent.click(screen.getByRole('button', { name }))
// Closed, there's no button: the cover opens by a click, a drag or the arrow keys.
const openBook = () => fireEvent.keyDown(book(), { key: 'ArrowRight' })
const pageBox = () => screen.getByRole('textbox', { name: 'Page' })
// jsdom lays nothing out, and a press on the book picks its direction from the half it lands on.
const sizeBook = () =>
  vi.spyOn(book(), 'getBoundingClientRect').mockReturnValue({ x: 0, y: 0, left: 0, top: 0, right: 400, bottom: 600, width: 400, height: 600, toJSON: () => ({}) })
const newcomer = (id: string, first: string): DirectoryEntry => ({ id, first_name: first, last_name: 'Désir', side: 'both', has_account: false, contacts: [] })

// A pointer event at a set time: the flick rule reads timeStamp, which fireEvent can't set (and React reads 0 as now).
const pointerAt = (type: 'pointerDown' | 'pointerMove' | 'pointerUp', clientX: number, timeStamp: number) => {
  const event = createEvent[type](book(), { pointerId: 1, button: 0, clientX, clientY: 300 })
  Object.defineProperty(event, 'timeStamp', { value: timeStamp })
  fireEvent(book(), event)
}

beforeEach(() => stubMedia())

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('Members', () => {
  it('opens on the closed cover, then the bookplate and Welcome', async () => {
    await renderPage()

    expect(position()).toBe('Click the cover, or drag it, to open the book')
    expect(within(book()).getByText('Family members')).toBeInTheDocument()
    openBook()

    expect(position()).toBe('Welcome')
    expect(within(book()).getByText('the Pluviose family')).toBeInTheDocument()
    expect(within(book()).getByRole('heading', { name: 'Everyone in our family, in one book' })).toBeInTheDocument()
    expect(within(book()).getByRole('link', { name: 'My profile ›' })).toHaveAttribute('href', '/profile')
  })

  it.each([
    { name: 'on the front cover', steps: [], says: 'Click the cover, or drag it, to open the book' },
    { name: 'on the back cover', steps: ['Jump to In Memory', 'Next ▶', 'Next ▶'], says: 'The end · click the back cover to go back' },
  ])('shows only how to open the book while it’s closed, $name', async ({ steps, says }) => {
    await renderPage()
    if (steps.length) {
      openBook()
      steps.forEach((step) => press(step))
    }

    expect(position()).toBe(says)
    expect(screen.getByText(says)).toBeVisible()
    expect(screen.queryByRole('button', { name: /Previous|Next/ })).toBeNull()
    expect(screen.queryByRole('textbox', { name: 'Page' })).toBeNull()
  })

  it.each([
    { name: 'goes to a page typed and entered', typed: '7', finish: 'enter', shows: 'Pages 6–7 · Naomie', box: '7' },
    { name: 'goes to a page typed when leaving the box', typed: '3', finish: 'blur', shows: 'Pages 2–3 · Fritz', box: '3' },
    { name: 'stays put for a page past the end', typed: '99', finish: 'enter', shows: 'Welcome', box: '1' },
    { name: 'stays put for page 0', typed: '0', finish: 'enter', shows: 'Welcome', box: '1' },
    { name: 'stays put when Escape drops what was typed', typed: '5', finish: 'escape', shows: 'Welcome', box: '1' },
  ])('the page box between the buttons $name', async ({ typed, finish, shows, box }) => {
    await renderPage()
    openBook()
    expect(pageBox()).toHaveValue('1')
    expect(screen.getByText('of 8')).toBeInTheDocument()

    fireEvent.change(pageBox(), { target: { value: typed } })
    if (finish === 'enter') {
      fireEvent.submit(pageBox().closest('form')!)
    } else {
      if (finish === 'escape') {
        fireEvent.keyDown(pageBox(), { key: 'Escape' })
      }
      fireEvent.blur(pageBox())
    }

    expect(position()).toBe(shows)
    expect(pageBox()).toHaveValue(box)
  })

  it('shows the page typed from the moment it’s entered, through the turn and after, until the book moves on', async () => {
    stubMedia({ reduced: false })
    await renderPage()
    vi.useFakeTimers()
    try {
      const wait = () =>
        act(() => {
          vi.advanceTimersByTime(920)
        })
      openBook()
      wait()

      fireEvent.change(pageBox(), { target: { value: '7' } })
      fireEvent.submit(pageBox().closest('form')!)
      expect(book()).toHaveClass(/animNext/)
      expect(pageBox()).toHaveValue('7')
      wait()
      expect(position()).toBe('Pages 6–7 · Naomie')
      expect(pageBox()).toHaveValue('7')

      press('◀ Previous')
      wait()
      expect(pageBox()).toHaveValue('4')
    } finally {
      vi.useRealTimers()
    }
  })

  it('lists what’s inside, with page numbers and letters, and no family sides', async () => {
    await renderPage()
    openBook()
    press('Next ▶')

    expect(position()).toBe('Pages 2–3 · Fritz')
    const inside = within(book())
    expect(inside.getByRole('button', { name: 'Members, A to Z, page 3' })).toBeInTheDocument()
    expect(inside.getByText('4 people with us')).toBeInTheDocument()
    expect(inside.getByRole('button', { name: 'In Memory, page 7' })).toBeInTheDocument()
    expect(inside.getByText('1 remembered')).toBeInTheDocument()
    expect(inside.queryByText(/\bsides?\b/i)).not.toBeInTheDocument()
    expect(inside.getByText('Married in · 1')).toBeInTheDocument()

    fireEvent.click(inside.getByRole('button', { name: 'Go to N' }))

    expect(position()).toBe('Pages 4–5 · Maya & Nadège')
  })

  it('shows what each person shares, and never a phone number or address', async () => {
    await renderPage()
    openBook()
    press('Next ▶')

    const fritz = within(book())
    expect(fritz.getByRole('heading', { name: 'Fritz Pluviose' })).toBeInTheDocument()
    expect(fritz.getByText('Sibling')).toBeInTheDocument()
    expect(fritz.queryByText(/\bsides?\b/i)).not.toBeInTheDocument()
    expect(fritz.getByRole('link', { name: 'Email Fritz' })).toHaveAttribute('href', 'mailto:fritz.p@example.com')
    expect(fritz.getByRole('link', { name: 'Fritz on YouTube' })).toHaveAttribute('href', 'https://www.youtube.com/@fritzpluviose')
    expect(fritz.getByRole('link', { name: /Phone number shared with you/ })).toHaveAttribute('href', '/people/fritz')
    expect(fritz.queryByText(/555/)).not.toBeInTheDocument()
    expect(fritz.queryByText('hidden@example.com')).not.toBeInTheDocument()
    expect(fritz.getByRole('link', { name: 'Open profile of Fritz Pluviose›' })).toHaveAttribute('href', '/people/fritz')
  })

  it('offers to ask for what isn’t shared, and shows your own page and someone not on the hub', async () => {
    await renderPage()
    openBook()
    press('Next ▶')
    press('Next ▶')

    const spread = within(book())
    expect(spread.getByText('This is your page.')).toBeInTheDocument()
    expect(spread.getByText('You')).toBeInTheDocument()
    expect(spread.getByRole('link', { name: 'Edit in My profile ›' })).toHaveAttribute('href', '/profile')
    expect(spread.getByRole('button', { name: 'Request address' })).toBeInTheDocument()
    expect(spread.getByText('Phone number requested')).toBeInTheDocument()

    press('Next ▶')

    expect(position()).toBe('Pages 6–7 · Naomie')
    expect(within(book()).getByText('Not on the hub yet, so there’s nothing to share here.')).toBeInTheDocument()
    expect(within(book()).getByText('Married in')).toBeInTheDocument()
  })

  it('remembers those who have passed, with no contact details', async () => {
    await renderPage()
    openBook()

    press('Jump to In Memory')
    expect(position()).toBe('Pages 6–7 · Naomie')
    press('Next ▶')

    const page = within(book())
    expect(position()).toBe('Page 8 · Joseph')
    expect(page.getByText('In loving memory')).toBeInTheDocument()
    expect(page.getByText('“Drove a yellow cab in New York.”')).toBeInTheDocument()
    expect(page.getByRole('link', { name: 'Open profile of Joseph Pluviose›' })).toHaveAttribute('href', '/people/joseph')
    expect(page.queryByRole('button', { name: /^Request/ })).not.toBeInTheDocument()
  })

  it('turns with the arrow keys, the letter tabs and the corners, and closes at the back cover', async () => {
    await renderPage()
    fireEvent.keyDown(book(), { key: 'ArrowRight' })
    expect(position()).toBe('Welcome')

    press('Jump to M')
    expect(position()).toBe('Pages 4–5 · Maya & Nadège')
    expect(screen.getByRole('button', { name: 'Jump to M' })).toHaveAttribute('aria-current', 'true')

    press('Next page')
    press('Next page')
    expect(position()).toBe('Page 8 · Joseph')
    press('Next ▶')
    expect(position()).toBe('The end · click the back cover to go back')

    fireEvent.keyDown(book(), { key: 'ArrowLeft' })
    expect(position()).toBe('Page 8 · Joseph')
  })

  it('doesn’t turn when a touch on the page becomes a scroll', async () => {
    await renderPage()
    openBook()

    fireEvent.pointerDown(book(), { pointerId: 1, button: 0, clientX: 600, clientY: 300 })
    fireEvent.pointerCancel(book(), { pointerId: 1 })

    expect(position()).toBe('Welcome')
  })

  it('animates a turn, then lands on the next spread', async () => {
    stubMedia({ reduced: false })
    await renderPage()
    vi.useFakeTimers()
    try {
      openBook()

      expect(book()).toHaveClass(/animNext/)
      act(() => {
        vi.advanceTimersByTime(920)
      })
      expect(book()).not.toHaveClass(/animNext/)
      expect(position()).toBe('Welcome')
    } finally {
      vi.useRealTimers()
    }
  })

  it('stays open at the same people when a refresh adds someone before them', async () => {
    let everyone = people
    const client = await renderPage(() => everyone)
    openBook()
    press('Jump to N')
    expect(position()).toBe('Pages 4–5 · Maya & Nadège')

    everyone = [...people, newcomer('alain', 'Alain'), newcomer('bea', 'Béa')]
    await act(() => client.refetchQueries({ queryKey: ['directory'] }))

    await waitFor(() => expect(position()).toBe('Pages 6–7 · Maya & Nadège'))
  })

  // One newcomer shifts every page after them to the other side of the spine; two wouldn't show the difference.
  it.each([
    { name: 'turned to', jumps: ['Jump to N'] },
    { name: 'already in view', jumps: ['Jump to M', 'Jump to N'] },
  ])('stays at the person jumped to, $name, when a refresh adds one person before them', async ({ jumps }) => {
    let everyone = people
    const client = await renderPage(() => everyone)
    openBook()
    jumps.forEach((jump) => press(jump))
    expect(position()).toBe('Pages 4–5 · Maya & Nadège')

    everyone = [...people, newcomer('alain', 'Alain')]
    await act(() => client.refetchQueries({ queryKey: ['directory'] }))

    await waitFor(() => expect(position()).toBe('Pages 6–7 · Nadège & Naomie'))
  })

  it('drops a turn under way when a refresh takes away the pages it was turning to', async () => {
    stubMedia({ reduced: false })
    let everyone = people
    const client = await renderPage(() => everyone)
    vi.useFakeTimers()
    try {
      openBook()
      act(() => {
        vi.advanceTimersByTime(920)
      })
      press('Jump to In Memory')
      act(() => {
        vi.advanceTimersByTime(920)
      })
      press('Next ▶')

      everyone = []
      await act(() => client.refetchQueries({ queryKey: ['directory'] }))
      // The refetched book lands first, as in a browser, where it comes long before the turn's end.
      act(() => {
        vi.advanceTimersByTime(1)
      })
      expect(within(book()).getByRole('heading', { name: 'In Memory' })).toBeInTheDocument()
      expect(book()).not.toHaveClass(/animNext/)
      act(() => {
        vi.advanceTimersByTime(920)
      })

      expect(within(book()).getByRole('heading', { name: 'In Memory' })).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it.each([
    { name: 'a letter on the Contents page', before: ['Next ▶'], control: () => within(book()).getByRole('button', { name: 'Go to N' }), keeps: false },
    { name: 'an entry on the Contents page', before: ['Next ▶'], control: () => within(book()).getByRole('button', { name: 'In Memory, page 7' }), keeps: false },
    { name: 'the corner that closes the book', before: [], control: () => screen.getByRole('button', { name: 'Previous page' }), keeps: false },
    { name: 'the Previous button that closes the book', before: [], control: () => screen.getByRole('button', { name: '◀ Previous' }), keeps: false },
    { name: 'the Next button under the book', before: [], control: () => screen.getByRole('button', { name: 'Next ▶' }), keeps: true },
  ])('turns with $name and leaves focus where the arrow keys still work', async ({ before, control, keeps }) => {
    await renderPage()
    openBook()
    before.forEach((step) => press(step))
    const button = control()
    const from = position()
    button.focus()

    fireEvent.click(button)

    expect(position()).not.toBe(from)
    expect(keeps ? button : book()).toHaveFocus()
  })

  it('turns a dragged page at once on letting go, with reduced motion, without it following the pointer', async () => {
    await renderPage()
    openBook()
    sizeBook()
    const pointer = { pointerId: 1, button: 0, clientY: 300 }

    fireEvent.pointerDown(book(), { ...pointer, clientX: 350 })
    fireEvent.pointerMove(book(), { ...pointer, clientX: 300 })
    fireEvent.pointerMove(book(), { ...pointer, clientX: 100 })
    expect(book().querySelector('[inert]')).toBeNull()
    fireEvent.pointerUp(book(), { ...pointer, clientX: 100 })

    expect(position()).toBe('Pages 2–3 · Fritz')
  })

  it('lets a page fall back when a drag under way becomes a scroll', async () => {
    await renderPage()
    openBook()
    sizeBook()
    const pointer = { pointerId: 1, button: 0, clientY: 300 }

    vi.useFakeTimers()
    try {
      fireEvent.pointerDown(book(), { ...pointer, clientX: 350 })
      fireEvent.pointerMove(book(), { ...pointer, clientX: 100 })
      fireEvent.pointerCancel(book(), { ...pointer, clientX: 100 })
      act(() => {
        vi.advanceTimersByTime(1000)
      })
    } finally {
      vi.useRealTimers()
    }

    expect(position()).toBe('Welcome')
  })

  it('still turns after reduced motion comes on in the middle of a drag', async () => {
    const setReduced = stubMedia({ reduced: false })
    await renderPage()
    vi.useFakeTimers()
    try {
      openBook()
      act(() => {
        vi.advanceTimersByTime(920)
      })
    } finally {
      vi.useRealTimers()
    }
    sizeBook()
    const pointer = { pointerId: 1, button: 0, clientY: 300 }
    fireEvent.pointerDown(book(), { ...pointer, clientX: 350 })
    fireEvent.pointerMove(book(), { ...pointer, clientX: 100 })
    expect(book().querySelector('[inert]')).not.toBeNull()

    act(() => setReduced(true))
    fireEvent.pointerUp(book(), { ...pointer, clientX: 100 })
    expect(position()).toBe('Pages 2–3 · Fritz')
    expect(book().querySelector('[inert]')).toBeNull()

    press('Next ▶')
    expect(position()).toBe('Pages 4–5 · Maya & Nadège')
  })

  // Dragged a quarter of the way, quickly: let go at once it's a flick; held still first, it falls back.
  it.each([
    { name: 'turns when flicked', upAt: 30, shows: 'Pages 2–3 · Fritz' },
    { name: 'falls back when held still before letting go', upAt: 2030, shows: 'Welcome' },
  ])('a page dragged a quarter of the way $name', async ({ upAt, shows }) => {
    await renderPage()
    openBook()
    sizeBook()

    pointerAt('pointerDown', 350, 1000)
    pointerAt('pointerMove', 340, 1010)
    pointerAt('pointerMove', 259, 1020)
    pointerAt('pointerUp', 259, 1000 + upAt)

    expect(position()).toBe(shows)
  })

  it('doesn’t pass a request made of one person to whoever takes their page after a refresh', async () => {
    let everyone = people
    const client = await renderPage(() => everyone)
    openBook()
    press('Jump to N')
    fireEvent.click(within(book()).getByRole('button', { name: 'Request address' }))
    await waitFor(() => expect(within(book()).getAllByText('Address requested')).not.toEqual([]))

    const nadia: DirectoryEntry = { ...newcomer('nadia', 'Nadia'), has_account: true, contacts: [{ kind: 'address', requestable: true, my_request_status: 'none' }] }
    everyone = [...people.filter((p) => p.id !== 'nadege'), nadia]
    await act(() => client.refetchQueries({ queryKey: ['directory'] }))

    await waitFor(() => expect(position()).toBe('Pages 4–5 · Maya & Nadia'))
    expect(within(book()).getByRole('button', { name: 'Request address' })).toBeInTheDocument()
    expect(within(book()).queryAllByText('Address requested')).toEqual([])
  })

  it('drops a scrub along the tabs when a refresh takes letters away', async () => {
    let everyone = people
    const client = await renderPage(() => everyone)
    openBook()
    // Each tab 100px below the last.
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      const top = 100 * Number(this.dataset.tab ?? 0)
      return { x: 0, y: top, left: 0, top, right: 20, bottom: top + 20, width: 20, height: 20, toJSON: () => ({}) }
    })
    const tabs = () => screen.getByRole('navigation', { name: 'Jump to a letter' })
    fireEvent.pointerDown(tabs(), { pointerId: 2, clientY: 10 })
    fireEvent.pointerMove(tabs(), { pointerId: 2, clientY: 310 })
    expect(position()).toBe('Pages 6–7 · Naomie')

    everyone = people.filter((p) => p.id === 'fritz')
    await act(() => client.refetchQueries({ queryKey: ['directory'] }))
    await waitFor(() => expect(position()).toBe('Pages 4–5 · Joseph'))
    fireEvent.pointerMove(tabs(), { pointerId: 2, clientY: 10 })
    fireEvent.pointerUp(tabs(), { pointerId: 2, clientY: 10 })

    expect(position()).toBe('Pages 4–5 · Joseph')
    expect(within(tabs()).getAllByRole('button').map((b) => b.textContent)).toEqual(['F', '♡'])
  })

  it.each([
    { name: 'opens the front cover tapped on its left half', toEnd: false, clientX: 100, shows: 'the Pluviose family' },
    { name: 'turns back from the back cover tapped on its right half', toEnd: true, clientX: 300, shows: 'Sa w plante, se li w rekòlte.' },
  ])('on a phone, $name', async ({ toEnd, clientX, shows }) => {
    stubMedia({ narrow: true })
    await renderPage()
    if (toEnd) {
      openBook()
      for (const step of ['Jump to In Memory', 'Next ▶', 'Next ▶', 'Next ▶']) {
        press(step)
      }
      expect(position()).toBe('The end · click the back cover to go back')
    }
    sizeBook()

    fireEvent.pointerDown(book(), { pointerId: 1, button: 0, clientX, clientY: 300 })
    fireEvent.pointerUp(book(), { pointerId: 1, button: 0, clientX, clientY: 300 })

    expect(within(book()).getByText(shows)).toBeInTheDocument()
  })

  it('asks for a detail without fetching the whole family again', async () => {
    await renderPage()
    openBook()
    press('Jump to M')
    press('Request address')

    expect(await within(book()).findAllByText('Address requested')).toHaveLength(2)
    expect(vi.mocked(fetch).mock.calls.filter(([url]) => url === '/api/directory')).toHaveLength(1)
  })

  it('shows one page at a time on a phone', async () => {
    stubMedia({ narrow: true })
    await renderPage()
    expect(position()).toBe('Click the cover, or drag it, to open the book')

    openBook()
    expect(position()).toBe('Welcome')
    press('Next ▶')
    expect(position()).toBe('Page 1')
    press('Jump to F')

    expect(position()).toBe('Page 3 · Fritz')
    expect(within(book()).queryByText('Maya Pluviose')).not.toBeInTheDocument()
  })
})
