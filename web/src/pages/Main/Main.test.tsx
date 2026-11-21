import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  )
})

afterEach(() => {
  vi.unstubAllGlobals()
})

function renderTree(personId: string | null, { url = '/tree', data = tree } = {}) {
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
// The card with its pills and quick look: where the mouse goes in and out.
const personOf = (name: string) => cardOf(name).closest<HTMLElement>('[data-person]')!
// The lines drawn green: the chain between the open quick look and the focus person.
const chain = () => [...document.querySelectorAll('[data-chain]')].map((p) => p.getAttribute('data-chain')).sort()

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
  })

  it('centers on a relative on a click', () => {
    renderTree('fritz')

    fireEvent.click(cardOf('Jean-Robert'))

    expect(screen.getByRole('region', { name: 'Family tree centered on Jean-Robert Pluviose' })).toBeInTheDocument()
    expect(screen.getByText('With Guerline')).toBeInTheDocument()
    expect(screen.getByText('Jean-Robert’s parents')).toBeInTheDocument()
    expect(screen.getByText('Family tree centered on Jean-Robert Pluviose')).toBeInTheDocument()

  })

  it('links every card to its person’s profile, with no pop-up to open first', () => {
    renderTree('fritz')

    for (const person of document.querySelectorAll<HTMLElement>('[data-person]')) {
      expect(within(person).getByRole('link', { name: /^View profile of .+ ›$/ })).toHaveAttribute('href', `/people/${person.dataset.person}`)
    }
    expect(screen.getByRole('link', { name: 'View profile of Jean-Robert Pluviose ›' })).toBeInTheDocument()

    fireEvent.pointerEnter(personOf('Jean-Robert'), { pointerType: 'mouse' })

    expect(screen.queryByText('Taught in Jacmel for forty years.')).not.toBeInTheDocument()
  })

  it('has no title, Back or Pause motion button around the tree', () => {
    renderTree('fritz')

    expect(screen.getByRole('heading', { level: 1, name: 'Family tree' })).toHaveClass('visually-hidden')
    expect(screen.queryByRole('button', { name: /motion|^Back$/ })).not.toBeInTheDocument()
  })

  it('goes back to you from a relative', () => {
    renderTree('fritz')
    fireEvent.click(cardOf('Jean-Robert'))

    fireEvent.click(screen.getByRole('button', { name: 'Back to me' }))

    expect(screen.getByRole('region', { name: 'Family tree centered on you' })).toBeInTheDocument()
  })

  it.each([
    { name: 'zooms in a step', press: ['Zoom in'], level: '110%', scale: 'scale(1.1)' },
    { name: 'zooms out a step', press: ['Zoom out'], level: '90%', scale: 'scale(0.9)' },
    { name: 'stops at 150%', press: Array(8).fill('Zoom in'), level: '150%', scale: 'scale(1.5)' },
    { name: 'stops at 50%', press: Array(8).fill('Zoom out'), level: '50%', scale: 'scale(0.5)' },
  ])('$name with the buttons, and keeps the zoom while centering', ({ press, level, scale }) => {
    renderTree('fritz')
    const stage = () => document.querySelector<HTMLElement>('[data-stage]')!

    for (const name of press) {
      const button = screen.getByRole('button', { name })
      if (!(button as HTMLButtonElement).disabled) {
        fireEvent.click(button)
      }
    }

    expect(screen.getByRole('button', { name: `Reset zoom, now ${level}` })).toBeInTheDocument()
    expect(stage().style.transform).toMatch(new RegExp(`${scale.replace(/[().]/g, '\\$&')}$`))
    fireEvent.click(cardOf('Jean-Robert'))
    expect(stage().style.transform).toMatch(new RegExp(`${scale.replace(/[().]/g, '\\$&')}$`))
  })

  it('moves the tree when dragged, and a drag from a card doesn’t center on them', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const stage = document.querySelector<HTMLElement>('[data-stage]')!
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }

    fireEvent.pointerDown(cardOf('Jean-Robert'), { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 120, clientY: 90 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 80 })
    fireEvent.pointerUp(panel, mouse)
    // A pointer's click has a click count in detail; a keyboard's has 0.
    fireEvent.click(cardOf('Jean-Robert'), { detail: 1 })

    expect(stage.style.transform).not.toBe('')
    expect(screen.getByRole('region', { name: 'Family tree centered on you' })).toBeInTheDocument()

    fireEvent.click(cardOf('Jean-Robert'), { detail: 1 })
    expect(screen.getByRole('region', { name: 'Family tree centered on Jean-Robert Pluviose' })).toBeInTheDocument()
  })

  it('still centers on a card from the keyboard after a drag the browser canceled', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }
    fireEvent.pointerDown(cardOf('Jean-Robert'), { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 80 })
    fireEvent.pointerCancel(panel, mouse)

    fireEvent.click(cardOf('Jean-Robert'), { detail: 0 })

    expect(screen.getByRole('region', { name: 'Family tree centered on Jean-Robert Pluviose' })).toBeInTheDocument()
  })

  it('doesn’t move the tree when the mouse comes back after a press released outside the panel', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }
    fireEvent.pointerDown(panel, { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerUp(window, mouse)

    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 130 })

    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toBe('')
    expect(screen.getByRole('button', { name: 'Recenter' })).toBeDisabled()
  })

  it('keeps the profile links from being dragged as links, so a drag from one moves the tree', () => {
    renderTree('fritz')

    for (const link of screen.getAllByRole('link', { name: /^View profile of / })) {
      expect(link).toHaveAttribute('draggable', 'false')
    }
  })

  it('recenters a tree that was moved, keeping its zoom', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }
    expect(screen.getByRole('button', { name: 'Recenter' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    fireEvent.pointerDown(panel, { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 130 })
    fireEvent.pointerUp(panel, mouse)

    fireEvent.click(screen.getByRole('button', { name: 'Recenter' }))

    expect(screen.getByRole('button', { name: 'Recenter' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Reset zoom, now 110%' })).toBeInTheDocument()
  })

  it('centers a tree that was moved when the zoom is reset', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }
    fireEvent.pointerDown(panel, { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 130 })
    fireEvent.pointerUp(panel, mouse)

    fireEvent.click(screen.getByRole('button', { name: /^Reset zoom/ }))

    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toBe('')
  })

  it.each([
    { name: 'up to the parent whose line continues', viewer: 'jonas', arrow: 'more: center the tree on Fritz Pluviose', next: 'Fritz Pluviose' },
    { name: 'down to the child whose children have children', viewer: 'joseph', arrow: 'more: center the tree on Jean-Robert Pluviose', next: 'Jean-Robert Pluviose' },
  ])('has a plain arrow at the panel’s edge that goes $name', ({ viewer, arrow, next }) => {
    renderTree(viewer)

    const button = screen.getByRole('button', { name: arrow })
    expect(button.textContent).toMatch(/^[▲▼] more:/)
    expect(button.closest('[data-person]')).toBeNull()
    fireEvent.click(button)

    expect(screen.getByRole('region', { name: `Family tree centered on ${next}` })).toBeInTheDocument()
  })

  it('walks back down the line the up arrow came by, even with nobody further below', () => {
    renderTree('jonas')

    fireEvent.click(screen.getByRole('button', { name: 'more: center the tree on Fritz Pluviose' }))
    expect(screen.getByRole('region', { name: 'Family tree centered on Fritz Pluviose' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'more: center the tree on Jonas Pluviose' }))
    expect(screen.getByRole('region', { name: 'Family tree centered on you' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'more: center the tree on Fritz Pluviose' }))
    fireEvent.click(cardOf('Jean-Robert'))
    expect(screen.queryByRole('button', { name: 'more: center the tree on Fritz Pluviose' })).not.toBeInTheDocument()
  })

  it('hides the arrows once the tree is moved by hand, and Recenter brings them back', () => {
    renderTree('jonas')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }

    fireEvent.pointerDown(panel, { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 130 })
    fireEvent.pointerUp(panel, mouse)
    expect(screen.queryByRole('button', { name: /^more:/ })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Recenter' }))
    expect(screen.getByRole('button', { name: /^more:/ })).toBeInTheDocument()
  })

  it('zooms with a two-finger pinch', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const finger = (pointerId: number, clientX: number) => ({ pointerId, pointerType: 'touch', clientX, clientY: 100 })

    fireEvent.pointerDown(panel, finger(1, 100))
    fireEvent.pointerDown(panel, finger(2, 200))
    fireEvent.pointerMove(panel, finger(2, 220))

    expect(screen.getByRole('button', { name: 'Reset zoom, now 120%' })).toBeInTheDocument()
  })

  it('zooms with Ctrl and the wheel, which is also a trackpad pinch, and resets', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })

    fireEvent.wheel(panel, { ctrlKey: true, deltaY: -200 })
    expect(screen.getByRole('button', { name: /^Reset zoom, now 150%$/ })).toBeInTheDocument()
    fireEvent.wheel(panel, { deltaY: -200 })
    expect(screen.getByRole('button', { name: /^Reset zoom, now 150%$/ })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /^Reset zoom/ }))

    expect(screen.getByRole('button', { name: 'Reset zoom, now 100%' })).toBeInTheDocument()
    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toBe('')
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

    fireEvent.click(cardOf('Jean-Robert'))

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

  it('raises only the generation whose card is lifted, for every card, so its shadow sits over the rows nearby', () => {
    renderTree('fritz')
    const people = [...document.querySelectorAll<HTMLElement>('[data-person]')]
    expect(people.length).toBeGreaterThan(5)

    for (const person of people) {
      fireEvent.pointerEnter(person, { pointerType: 'mouse' })
      const raised = [...document.querySelectorAll('[data-row]')].filter((r) => /raised/.test(r.className))
      expect(raised, person.dataset.person).toEqual([person.closest('[data-row]')])
      fireEvent.pointerLeave(person, { pointerType: 'mouse' })
    }
  })

  it('numbers the generations as shown, from the top, when the ones above the focus are missing', () => {
    renderTree('joseph')

    const rows = [...document.querySelectorAll<HTMLElement>('[data-row]')].map((r) => [r.dataset.row, r.firstElementChild!.textContent])
    expect(rows).toEqual([
      ['0', 'Your household & siblings'],
      ['1', 'Your children'],
      ['2', 'Grandchildren'],
    ])
  })

  it('brings the focus person’s card into view on opening the page', () => {
    const scrolled: Element[] = []
    // A laid-out tree with the card below the fold: jsdom gives everything no size and puts it at 0.
    vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(1000)
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue(DOMRect.fromRect({ y: 2000, height: 150 }))
    Element.prototype.scrollIntoView = function (this: Element) {
      scrolled.push(this)
    }
    renderTree('fritz')

    expect(scrolled).toEqual([cardOf('Fritz').closest('[data-card]')])
    delete (Element.prototype as Partial<Element>).scrollIntoView
    vi.restoreAllMocks()
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

  // Inline, where the page sets them: jsdom's computed style has no animations.
  it.each([
    { name: 'Grandparents', delay: '0ms' },
    { name: 'Your parents', delay: '80ms' },
    { name: 'Your household & siblings', delay: '160ms' },
    { name: 'Your children', delay: '240ms' },
  ])('brings in “$name” at $delay, top to bottom', ({ name, delay }) => {
    renderTree('fritz')

    expect(screen.getByText(name).parentElement!.style.animationDelay).toBe(delay)
  })

  it.each([
    { line: 'up:jr', into: 'the parents', delay: '480ms' },
    { line: 'sib:maya', into: 'a sibling', delay: '560ms' },
    { line: 'kids:0', into: 'the children', delay: '640ms' },
  ])('draws the line into $into once their generation has landed, at $delay', ({ line, delay }) => {
    renderTree('fritz')

    expect(document.querySelector<SVGElement>(`[data-line="${line}"]`)!.style.animationDelay).toBe(delay)
  })

  it.each([
    { name: 'a grandparent, through their child', viewer: 'fritz', card: 'Joseph', data: tree, lit: ['sib:fritz', 'up:jr'] },
    { name: 'a parent', viewer: 'fritz', card: 'Nadège', data: tree, lit: ['sib:fritz'] },
    { name: 'a sibling, through the parents', viewer: 'fritz', card: 'Maya', data: tree, lit: ['sib:fritz', 'sib:maya'] },
    { name: 'a partner, sharing the household', viewer: 'fritz', card: 'Fabienne', data: tree, lit: [] },
    { name: 'a child in the second group', viewer: 'fritz', card: 'Tim', data: tree, lit: ['kids:1'] },
    { name: 'a grandchild, through their parent', viewer: 'rose', card: 'Naomie', data: fostering, lit: ['down:mt', 'kids:0'] },
  ])('turns green the lines between the focus person and $name', ({ viewer, card, data, lit }) => {
    renderTree(viewer, { data })
    expect(chain()).toEqual([])

    fireEvent.pointerEnter(personOf(card), { pointerType: 'mouse' })

    expect(chain()).toEqual(lit)
  })

  it('draws a green line in with the rest, so a card already under the pointer lights it as it draws', () => {
    renderTree('fritz')

    fireEvent.pointerEnter(personOf('Joseph'), { pointerType: 'mouse' })

    expect(document.querySelector<SVGElement>('[data-chain="up:jr"]')!.style.animationDelay).toBe('480ms')
  })
})
