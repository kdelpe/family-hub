import { focusManager, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { familyKeys } from '../../api/family'
import { treeKeys } from '../../api/tree'
import { Main } from './Main'

const person = (id: string, first_name: string, gender: 'female' | 'male', bio_excerpt: string | null = null) => ({
  id,
  first_name,
  last_name: 'Pluviose',
  side: 'first',
  gender,
  is_deceased: false,
  bio_excerpt,
})
const parent = (parent_id: string, child_id: string, subtype = 'biological') => ({ id: `${parent_id}-${child_id}`, type: 'parent_child', subtype, parent_id, child_id })

// Fritz with his parents, his sister Maya, his half-sister Sabine, his wife Fabienne and their sons.
const tree = {
  persons: [
    person('fritz', 'Fritz', 'male'),
    person('jr', 'Jean-Robert', 'male', 'Taught in Jacmel for forty years.'),
    person('nadege', 'Nadège', 'female'),
    person('guerline', 'Guerline', 'female'),
    person('maya', 'Maya', 'female'),
    person('sabine', 'Sabine', 'female'),
    person('fabienne', 'Fabienne', 'female'),
    person('jonas', 'Jonas', 'male'),
    person('tim', 'Tim', 'male'),
    person('joseph', 'Joseph', 'male'),
    person('odette', 'Odette', 'female'),
  ],
  relationships: [
    parent('joseph', 'jr'),
    ...['fritz', 'maya'].flatMap((c) => [parent('jr', c), parent('nadege', c)]),
    parent('jr', 'sabine'),
    parent('guerline', 'sabine'),
    parent('fritz', 'jonas'),
    parent('fabienne', 'jonas'),
    parent('fritz', 'tim', 'adoptive'),
    { id: 'c1', type: 'couple', subtype: 'married', ended: false, person_ids: ['fritz', 'fabienne'] },
    { id: 'c2', type: 'couple', subtype: 'married', ended: true, person_ids: ['jr', 'guerline'] },
    { id: 'c3', type: 'couple', subtype: 'married', ended: true, person_ids: ['fritz', 'odette'] },
  ],
}

// Rose fosters her grandson Peterson and was fostered by Odette; her daughter Marie-Thérèse fosters Naomie.
const fostering = {
  persons: [
    person('rose', 'Rose', 'female'),
    person('frantz', 'Frantz', 'male'),
    person('mt', 'Marie-Thérèse', 'female'),
    person('mirlande', 'Mirlande', 'female'),
    person('peterson', 'Peterson', 'male'),
    person('odette', 'Odette', 'female'),
    person('naomie', 'Naomie', 'female'),
  ],
  relationships: [
    parent('rose', 'frantz'),
    parent('rose', 'mt'),
    parent('frantz', 'peterson'),
    parent('mirlande', 'peterson'),
    parent('rose', 'peterson', 'foster'),
    parent('odette', 'rose', 'foster'),
    parent('mt', 'naomie', 'foster'),
  ],
}

const json = (body: unknown) => new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } })
const row = (id: string, kind: string, value: string, visibility: string, platform: string | null = null, shared_with_you = false) => ({
  id,
  kind,
  platform,
  label: null,
  value,
  visibility,
  href: null,
  shared_with_you,
})

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  )
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => json({ contacts: [] })),
  )
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  focusManager.setFocused(undefined)
})

function renderTree(personId: string | null, { url = '/tree', data = tree } = {}) {
  // Fresh for the whole test: only the quick look's contacts are fetched.
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u1', email: 'fritz@example.com', role: 'member', person_id: personId })
  client.setQueryData(treeKeys.all, data)
  client.setQueryData(familyKeys.summary, { people: 10, generations: 4, earliest_record: null })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={createMemoryRouter([{ path: '/tree', element: <Main /> }], { initialEntries: [url] })} />
    </QueryClientProvider>,
  )
}

const cardOf = (name: string) => screen.getByRole('button', { name: new RegExp(`^${name} Pluviose`) })
const trail = () => within(screen.getByRole('navigation', { name: "Where you've been" }))

describe('Main', () => {
  it('centers on the viewer with their family around them', () => {
    renderTree('fritz')

    expect(screen.getByRole('region', { name: 'Family tree centered on you' })).toBeInTheDocument()
    expect(screen.getByText('Your parents')).toBeInTheDocument()
    expect(screen.getByText('Your household & siblings')).toBeInTheDocument()
    expect(cardOf('Fritz')).toHaveTextContent('You')
    expect(cardOf('Sabine')).toHaveTextContent('Half-sister')
    expect(cardOf('Tim')).toHaveTextContent('Son · adopted')
    expect(cardOf('Odette')).toHaveTextContent('Former wife')
    expect(screen.getByText((_, el) => el?.textContent === '4 generations')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back' })).toBeDisabled()
  })

  it('centers on a relative on double-click, then goes back', () => {
    renderTree('fritz')

    fireEvent.doubleClick(cardOf('Jean-Robert'))

    expect(screen.getByRole('region', { name: 'Family tree centered on Jean-Robert Pluviose' })).toBeInTheDocument()
    expect(screen.getByText('With Guerline')).toBeInTheDocument()
    expect(screen.getByText('Jean-Robert’s parents')).toBeInTheDocument()
    expect(screen.getByText('Family tree centered on Jean-Robert Pluviose')).toBeInTheDocument()
    expect(trail().getByRole('button', { name: 'Me' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Back' }))

    expect(screen.getByRole('region', { name: 'Family tree centered on you' })).toBeInTheDocument()
  })

  it('opens a quick look on click and closes it on a second click', async () => {
    renderTree('fritz')

    fireEvent.click(cardOf('Jean-Robert'))

    expect(await screen.findByText('Taught in Jacmel for forty years.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'View profile ›' })).toHaveAttribute('href', '/people/jr')

    fireEvent.click(cardOf('Jean-Robert'))

    await vi.waitFor(() => expect(screen.queryByText('Taught in Jacmel for forty years.')).not.toBeInTheDocument())
  })

  it('shows only a contact the family can see in the quick look', async () => {
    // A new Response each call: a body can be read only once.
    vi.mocked(fetch).mockImplementation(async () =>
      json({
        contacts: [
          row('c1', 'email', 'hidden@example.com', 'hidden'),
          row('c2', 'social', '+1 555 0100', 'on_request', 'whatsapp', true),
          row('c3', 'email', 'jr@example.com', 'family'),
        ],
      }),
    )
    renderTree('fritz')

    fireEvent.click(cardOf('Jean-Robert'))

    expect(await screen.findByText('Email · jr@example.com')).toBeInTheDocument()
    expect(screen.queryByText(/hidden@example.com/)).not.toBeInTheDocument()
    expect(screen.queryByText(/555 0100/)).not.toBeInTheDocument()
  })

  it('finds a relative without accents and centers on them', () => {
    renderTree('fritz')

    fireEvent.change(screen.getByRole('searchbox', { name: 'Find a relative' }), { target: { value: 'nadege' } })
    fireEvent.click(within(screen.getByRole('list', { name: 'Matching relatives' })).getByRole('button', { name: /Nadège Pluviose/ }))

    expect(screen.getByRole('region', { name: 'Family tree centered on Nadège Pluviose' })).toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'Matching relatives' })).not.toBeInTheDocument()
  })

  it('keeps the viewer’s relation on a former partner of someone else', () => {
    renderTree('fritz')

    fireEvent.doubleClick(cardOf('Jean-Robert'))

    expect(cardOf('Guerline')).toHaveTextContent('Former stepmother')
  })

  it('closes the matches when focus leaves the search', () => {
    renderTree('fritz')
    const search = screen.getByRole('searchbox', { name: 'Find a relative' })

    fireEvent.focus(search)
    fireEvent.change(search, { target: { value: 'sab' } })
    expect(screen.getByRole('list', { name: 'Matching relatives' })).toBeInTheDocument()

    fireEvent.blur(search, { relatedTarget: document.body })
    expect(screen.queryByRole('list', { name: 'Matching relatives' })).not.toBeInTheDocument()
  })

  it('never calls the viewer a half-sibling', () => {
    renderTree('sabine')

    fireEvent.change(screen.getByRole('searchbox', { name: 'Find a relative' }), { target: { value: 'maya' } })
    fireEvent.click(within(screen.getByRole('list', { name: 'Matching relatives' })).getByRole('button', { name: /Maya Pluviose/ }))

    expect(cardOf('Sabine')).toHaveTextContent(/You$/)
    expect(cardOf('Fritz')).toHaveTextContent('Half-brother')
  })

  it('asks a login linked to nobody to find a relative', () => {
    renderTree(null)

    expect(screen.getByText('Your login isn’t linked to anyone in the tree yet. Find a relative above to start.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Back to me' })).not.toBeInTheDocument()
  })

  it.each([
    { name: 'the person a profile asked for', viewer: 'fritz', url: '/tree?person=nadege', centered: 'Nadège Pluviose' },
    { name: 'you, when that person isn’t in the tree', viewer: 'fritz', url: '/tree?person=nobody', centered: 'you' },
    { name: 'the person asked for, for a login linked to nobody', viewer: null, url: '/tree?person=maya', centered: 'Maya Pluviose' },
  ])('opens on $name', ({ viewer, url, centered }) => {
    renderTree(viewer, { url })

    expect(screen.getByRole('region', { name: `Family tree centered on ${centered}` })).toBeInTheDocument()
  })

  it.each([
    { name: 'while it’s read again', next: () => new Promise<Response>(() => {}) },
    { name: 'once reading it again failed', next: () => Promise.reject(new TypeError('Failed to fetch')) },
  ])('hides the shared contact $name, since it may be hidden by now', async ({ next }) => {
    let contacts = async () => json({ contacts: [row('c3', 'email', 'jr@example.com', 'family')] })
    vi.mocked(fetch).mockImplementation((url) => (String(url).endsWith('/contacts') ? contacts() : Promise.resolve(json({}))))
    renderTree('fritz')
    fireEvent.click(cardOf('Jean-Robert'))
    expect(await screen.findByText('Email · jr@example.com')).toBeInTheDocument()
    contacts = next

    // Coming back to the window reads the contacts again.
    act(() => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })

    await vi.waitFor(() => expect(screen.queryByText('Email · jr@example.com')).not.toBeInTheDocument())
    expect(screen.getByText('Taught in Jacmel for forty years.')).toBeInTheDocument()
  })

  it.each([
    { name: 'Escape', dismiss: () => fireEvent.keyDown(document, { key: 'Escape' }) },
    { name: 'a press elsewhere', dismiss: () => fireEvent.pointerDown(document.body) },
  ])('lets $name cancel a click still waiting to open the quick look', ({ dismiss }) => {
    vi.useFakeTimers()
    renderTree('fritz')

    fireEvent.click(cardOf('Jean-Robert'))
    dismiss()
    act(() => vi.advanceTimersByTime(500))

    expect(screen.queryByRole('link', { name: 'View profile ›' })).not.toBeInTheDocument()
  })

  it('sends focus back to the card when Escape closes its quick look from inside', async () => {
    renderTree('fritz')
    fireEvent.click(cardOf('Jean-Robert'))
    const link = await screen.findByRole('link', { name: 'View profile ›' })
    link.focus()

    fireEvent.keyDown(link, { key: 'Escape' })

    expect(cardOf('Jean-Robert')).toHaveFocus()
    // Quietly: that focus doesn't open it again.
    expect(screen.queryByRole('link', { name: 'View profile ›' })).not.toBeInTheDocument()
  })

  it('lands keyboard focus on the tree once a first relative is picked from the search', () => {
    renderTree(null)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Find a relative' }), { target: { value: 'maya' } })
    const match = within(screen.getByRole('list', { name: 'Matching relatives' })).getByRole('button', { name: /Maya Pluviose/ })
    act(() => match.focus())

    // The pressed result goes away with the search it came from.
    fireEvent.click(match)

    expect(cardOf('Maya')).toHaveFocus()
  })

  it.each([
    { name: 'a foster child who is also a grandchild', viewer: 'rose', card: 'Peterson', sub: 'Grandson · foster' },
    { name: 'a foster parent who is also a grandmother', viewer: 'peterson', card: 'Rose', sub: 'Grandmother · foster' },
    { name: 'a foster sibling who is also a nephew', viewer: 'mt', card: 'Peterson', sub: 'Nephew · foster sibling' },
    { name: 'a grandparent only by fostering', viewer: 'peterson', card: 'Odette', sub: 'Foster parent' },
    { name: 'a grandchild only by fostering', viewer: 'rose', card: 'Naomie', sub: 'Foster child' },
  ])('says a foster link a closer relation hides: $name', ({ viewer, card, sub }) => {
    renderTree(viewer, { data: fostering })

    expect(cardOf(card)).toHaveTextContent(sub)
  })
})
