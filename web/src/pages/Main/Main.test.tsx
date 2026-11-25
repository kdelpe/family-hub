import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { familyKeys } from '../../api/family'
import { treeKeys } from '../../api/tree'
import { Main } from './Main'
import styles from './Main.module.css'

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
  vi.restoreAllMocks()
})

// A laid-out panel and tree: jsdom gives everything no size and puts it at 0. The panel is 600px square; the stage,
// the cards (190px wide) and the groups of grandparents get the heights given, and the grandparents' groups and
// Fritz's card their places.
function layOut({ tree, card = 150, grandparentsTop = 0, focusTop = 0 }: { tree: number; card?: number; grandparentsTop?: number; focusTop?: number }) {
  vi.spyOn(Element.prototype, 'clientWidth', 'get').mockReturnValue(600)
  vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockImplementation(function (this: HTMLElement) {
    return this.matches('[data-card]') ? 190 : 0
  })
  vi.spyOn(Element.prototype, 'clientHeight', 'get').mockReturnValue(600)
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(function (this: HTMLElement) {
    return this.matches('[data-stage]') ? tree : this.matches('[data-card], [data-unit^="up-"]') ? card : 0
  })
  vi.spyOn(HTMLElement.prototype, 'offsetTop', 'get').mockImplementation(function (this: HTMLElement) {
    return this.matches('[data-unit^="up-"]') ? grandparentsTop : this.matches('[data-person="fritz"] [data-card]') ? focusTop : 0
  })
}

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
  return client
}

// An arrow at the panel's edge, by the person it centers the tree on.
const arrowTo = (name: string) => new RegExp(`: center the tree on ${name}$`)
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

  it('brings a moved tree back to the middle on Back to me when it’s already centered on you, keeping the zoom', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    fireEvent.pointerDown(panel, { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 130 })
    fireEvent.pointerUp(panel, mouse)
    expect(screen.getByRole('button', { name: 'Recenter' })).toBeEnabled()

    fireEvent.click(screen.getByRole('button', { name: 'Back to me' }))

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
    { name: 'up to the parent whose line continues', viewer: 'jonas', next: 'Fritz Pluviose', words: '▲ Fritz’s parents' },
    // The grandparents are just above the panel's top, so the arrow shows though their own line ends with them.
    { name: 'up to a parent whose parents have none above them', viewer: 'fritz', next: 'Jean-Robert Pluviose', words: '▲ Jean-Robert’s parents' },
    { name: 'down to the child whose children have children', viewer: 'joseph', next: 'Jean-Robert Pluviose', words: '▼ Jean-Robert’s grandchildren' },
  ])('has an arrow at the panel’s edge naming the family it goes $name', ({ viewer, next, words }) => {
    renderTree(viewer)

    const button = screen.getByRole('button', { name: arrowTo(next) })
    expect(button.textContent).toBe(`${words}: center the tree on ${next}`)
    expect(button.closest('[data-person]')).toBeNull()
    fireEvent.click(button)

    expect(screen.getByRole('region', { name: `Family tree centered on ${next}` })).toBeInTheDocument()
  })

  it('walks back down the line the up arrow came by, even with nobody further below', () => {
    renderTree('jonas')

    fireEvent.click(screen.getByRole('button', { name: arrowTo('Fritz Pluviose') }))
    expect(screen.getByRole('region', { name: 'Family tree centered on Fritz Pluviose' })).toBeInTheDocument()

    const back = screen.getByRole('button', { name: arrowTo('Jonas Pluviose') })
    expect(back.textContent).toMatch(/^▼ Back to Jonas:/)
    fireEvent.click(back)
    expect(screen.getByRole('region', { name: 'Family tree centered on you' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: arrowTo('Fritz Pluviose') }))
    fireEvent.click(cardOf('Jean-Robert'))
    expect(screen.queryByRole('button', { name: arrowTo('Fritz Pluviose') })).not.toBeInTheDocument()
  })

  it('forgets the arrows’ way back when the person already centered is picked again', () => {
    renderTree('jonas')
    fireEvent.click(screen.getByRole('button', { name: arrowTo('Fritz Pluviose') }))
    expect(screen.getByRole('button', { name: arrowTo('Jonas Pluviose') })).toBeInTheDocument()

    fireEvent.click(cardOf('Fritz'))

    expect(screen.getByRole('region', { name: 'Family tree centered on Fritz Pluviose' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: arrowTo('Jonas Pluviose') })).not.toBeInTheDocument()
  })

  it('drops the arrows’ way back once someone on it has left the tree', async () => {
    const client = renderTree('fritz')
    fireEvent.click(cardOf('Jonas'))
    fireEvent.click(screen.getByRole('button', { name: arrowTo('Fritz Pluviose') }))
    expect(screen.getByRole('button', { name: arrowTo('Jonas Pluviose') })).toBeInTheDocument()

    act(() => {
      client.setQueryData(treeKeys.all, {
        persons: tree.persons.filter((p) => p.id !== 'jonas'),
        relationships: tree.relationships.filter((r) => !('child_id' in r) || r.child_id !== 'jonas'),
      })
    })

    await vi.waitFor(() => expect(screen.queryByRole('button', { name: arrowTo('Jonas Pluviose') })).not.toBeInTheDocument())
    expect(screen.getByRole('region', { name: 'Family tree centered on you' })).toBeInTheDocument()
  })

  it('hides the arrows once the tree is moved by hand, and Recenter brings them back', () => {
    renderTree('jonas')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }

    fireEvent.pointerDown(panel, { ...mouse, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(panel, { ...mouse, clientX: 140, clientY: 130 })
    fireEvent.pointerUp(panel, mouse)
    expect(screen.queryByRole('button', { name: /: center the tree on / })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Recenter' }))
    expect(screen.getByRole('button', { name: /: center the tree on / })).toBeInTheDocument()
  })

  it('moves the tree to show a card tabbed to outside the panel', () => {
    layOut({ tree: 1000 })
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const card = cardOf('Sabine').closest('[data-card]')!
    vi.spyOn(panel, 'getBoundingClientRect').mockReturnValue(DOMRect.fromRect({ x: 0, y: 0, width: 600, height: 800 }))
    vi.spyOn(card, 'getBoundingClientRect').mockReturnValue(DOMRect.fromRect({ x: 700, y: 100, width: 190, height: 156 }))

    act(() => cardOf('Sabine').focus())

    expect(screen.getByRole('button', { name: 'Recenter' })).toBeEnabled()
    vi.restoreAllMocks()
  })

  it('doesn’t move the tree for the page’s own focus on the person just centered on', () => {
    vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(1000)
    vi.spyOn(Element.prototype, 'clientWidth', 'get').mockReturnValue(600)
    // Every card outside the panel, so any reaction to that focus would show.
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
      return this.matches('section') ? DOMRect.fromRect({ width: 600, height: 800 }) : DOMRect.fromRect({ x: 700, width: 190, height: 156 })
    })
    renderTree('fritz')

    fireEvent.click(cardOf('Jean-Robert'))

    expect(document.activeElement).toBe(cardOf('Jean-Robert'))
    expect(screen.getByRole('button', { name: 'Recenter' })).toBeDisabled()
    vi.restoreAllMocks()
  })

  it('turns the browser scrolling the panel, to show a card tabbed to, into a move of the tree', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })

    panel.scrollLeft = 120
    fireEvent.scroll(panel)

    expect([panel.scrollLeft, panel.scrollTop]).toEqual([0, 0])
    expect(screen.getByRole('button', { name: 'Recenter' })).toBeEnabled()
  })

  it('zooms with a two-finger pinch', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const finger = (pointerId: number, clientX: number) => ({ pointerId, pointerType: 'touch', clientX, clientY: 100 })

    fireEvent.pointerDown(panel, finger(1, 100))
    fireEvent.pointerDown(panel, finger(2, 200))
    // Both fingers in the same frame: the second move builds on the first.
    act(() => {
      panel.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, ...finger(1, 90) }))
      panel.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, ...finger(2, 210) }))
    })

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

  it.each([
    { name: 'the right arrow moves it', key: 'ArrowRight', moved: true, zoom: '100%' },
    { name: 'the up arrow moves it', key: 'ArrowUp', moved: true, zoom: '100%' },
    { name: '+ zooms in', key: '+', moved: false, zoom: '110%' },
    { name: '− zooms out', key: '-', moved: false, zoom: '90%' },
    { name: 'a key with Ctrl is left to the browser', key: '+', ctrlKey: true, moved: false, zoom: '100%' },
  ])('from a card in the tree, $name', ({ key, ctrlKey = false, moved, zoom }) => {
    renderTree('fritz')

    const handled = !fireEvent.keyDown(cardOf('Maya'), { key, ctrlKey })

    expect(handled).toBe(moved || zoom !== '100%')
    expect(screen.getByRole('button', { name: 'Recenter' })).toHaveProperty('disabled', !moved)
    expect(screen.getByRole('button', { name: `Reset zoom, now ${zoom}` })).toBeInTheDocument()
  })

  it('glides the tree back on Recenter, and a move by hand takes over at once', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const stage = () => document.querySelector<HTMLElement>('[data-stage]')!
    vi.useFakeTimers()
    try {
      fireEvent.wheel(panel, { deltaY: 50 })
      expect(stage()).not.toHaveClass(/glide/)

      fireEvent.click(screen.getByRole('button', { name: 'Recenter' }))
      expect(stage()).toHaveClass(/glide/)
      act(() => {
        vi.advanceTimersByTime(300)
      })
      expect(stage()).not.toHaveClass(/glide/)

      // Already centered at 100%, a reset has nothing to glide.
      fireEvent.click(screen.getByRole('button', { name: /^Reset zoom/ }))
      expect(stage()).not.toHaveClass(/glide/)

      fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
      fireEvent.click(screen.getByRole('button', { name: /^Reset zoom/ }))
      expect(stage()).toHaveClass(/glide/)
      // A second reset mid-glide lets it carry on rather than jump to the end.
      fireEvent.click(screen.getByRole('button', { name: /^Reset zoom/ }))
      expect(stage()).toHaveClass(/glide/)
      fireEvent.wheel(panel, { deltaY: 50 })
      expect(stage()).not.toHaveClass(/glide/)
    } finally {
      vi.useRealTimers()
    }
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

  // The page doesn't scroll: the panel fills the window, and its top edge is where the grandparents' cards end, so
  // nothing shows cut off there.
  it.each([
    { name: 'starts at the bottom of the grandparents’ cards', who: 'fritz', transform: 'translate(0px, -350px) scale(1)' },
    { name: 'starts at its own top with no grandparents to show', who: 'jr', transform: '' },
  ])('the tree $name', ({ who, transform }) => {
    layOut({ tree: 2000, grandparentsTop: 200 })
    const scrollTo = vi.fn()
    vi.stubGlobal('scrollTo', scrollTo)

    renderTree(who)

    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toBe(transform)
    expect(scrollTo).not.toHaveBeenCalled()
  })

  it.each([
    { name: 'leaves room above the parents’ row under the grandparents, clear of the more arrow', who: 'fritz', spaced: true },
    { name: 'leaves no extra room above parents who are the first row', who: 'jr', spaced: false },
  ])('$name', ({ who, spaced }) => {
    renderTree(who)

    const parents = screen.getByText('Your parents').closest('[data-row]')!
    expect(parents.className.includes('spaced')).toBe(spaced)
  })

  it('moves the tree with a plain wheel or two-finger scroll, instead of scrolling the page', () => {
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })

    expect(fireEvent.wheel(panel, { deltaX: 10, deltaY: 50 })).toBe(false)

    expect(screen.getByRole('button', { name: 'Recenter' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Reset zoom, now 100%' })).toBeInTheDocument()
  })

  // A window too short or narrow for the page to fit scrolls the page, and up and down are left to it. Told by the same
  // media query as the CSS's, not the page's height: a long menu beside the tree doesn't count.
  const scrollingPage = (scrolls: boolean) =>
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: scrolls && query.includes('max-height: 519.98px'),
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }))
  const swipe = (panel: HTMLElement) => {
    const finger = (clientY: number) => ({ pointerId: 1, pointerType: 'touch', clientX: 100, clientY })
    fireEvent.pointerDown(panel, finger(100))
    fireEvent.pointerMove(panel, finger(110))
    return fireEvent.pointerMove(panel, finger(160))
  }
  it.each([
    { name: 'leaves a plain wheel to a page that scrolls', scrolls: true, gesture: (panel: HTMLElement) => fireEvent.wheel(panel, { deltaY: 50 }), movesDown: false },
    { name: 'leaves an up-and-down swipe to a page that scrolls', scrolls: true, gesture: swipe, movesDown: false },
    { name: 'moves the tree with a plain wheel beside a menu longer than the window', scrolls: false, gesture: (panel: HTMLElement) => fireEvent.wheel(panel, { deltaY: -50 }), movesDown: true },
  ])('$name', ({ scrolls, gesture, movesDown }) => {
    layOut({ tree: 600 })
    scrollingPage(scrolls)
    vi.spyOn(document.documentElement, 'scrollHeight', 'get').mockReturnValue(window.innerHeight + 200)
    renderTree('jr')

    const handled = !gesture(screen.getByRole('region', { name: 'Family tree centered on you' }))

    expect(handled).toBe(movesDown)
    // jsdom writes translate(x) alone when y is 0.
    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform.includes(', 50px)')).toBe(movesDown)
  })

  it('moves up from the grandparents’ cards to show the focus person’s card, zoomed in on a short panel', () => {
    // Grandparents end 350px down, Fritz's card 700–850px; at 150% the card would end 750px down a 600px panel.
    layOut({ tree: 2000, grandparentsTop: 200, focusTop: 700 })
    renderTree('fritz')
    for (let i = 0; i < 5; i++) {
      fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    }

    // Its bottom 16px above the panel's: 600 − 16 − 850 × 1.5.
    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toBe('translate(-150px, -691px) scale(1.5)')
  })

  it('takes over a glide from where the tree is shown, part way, at the zoom shown', () => {
    layOut({ tree: 2000 })
    renderTree('fritz')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    fireEvent.wheel(panel, { deltaY: 100 })
    fireEvent.click(screen.getByRole('button', { name: /^Reset zoom/ }))
    // Part way back: jsdom has no transitions, so the stage reports what a browser would show then.
    vi.stubGlobal(
      'DOMMatrixReadOnly',
      class {
        a = 1.1
        e = -30
        f = -80
      },
    )

    fireEvent.wheel(panel, { deltaY: 0 })

    expect(screen.getByRole('button', { name: 'Reset zoom, now 110%' })).toBeInTheDocument()
    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toBe('translate(-30px, -80px) scale(1.1)')
  })

  it('zooms about the panel’s middle from a glide taken over, as from any moved tree', () => {
    layOut({ tree: 2000 })
    renderTree('fritz')
    fireEvent.wheel(screen.getByRole('region', { name: 'Family tree centered on you' }), { deltaY: 100 })
    fireEvent.click(screen.getByRole('button', { name: 'Recenter' }))
    vi.stubGlobal(
      'DOMMatrixReadOnly',
      class {
        a = 1
        e = -30
        f = -40
      },
    )

    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))

    // The panel's middle (300, 300) stays put: 300 − (300 + 30) × 1.1, 300 − (300 + 40) × 1.1.
    const [x, y, z] = document.querySelector<HTMLElement>('[data-stage]')!.style.transform.match(/-?[\d.]+/g)!.map(Number)
    expect([x, y, z].map((n) => Math.round(n! * 100) / 100)).toEqual([-63, -74, 1.1])
  })

  it.each([
    { name: 'brings the card a keyboard pick lands on into the window, where the page scrolls', scrolls: true },
    { name: 'leaves the window alone where the page doesn’t scroll', scrolls: false },
  ])('$name', ({ scrolls }) => {
    scrollingPage(scrolls)
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue(DOMRect.fromRect({ y: 2000, height: 150 }))
    const scrollBy = vi.fn()
    vi.stubGlobal('scrollBy', scrollBy)
    renderTree(null)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Find a relative' }), { target: { value: 'maya' } })
    const match = within(screen.getByRole('list', { name: 'Matching relatives' })).getByRole('button', { name: /Maya Pluviose/ })
    act(() => match.focus())

    fireEvent.click(match)

    expect(cardOf('Maya')).toHaveFocus()
    // Where the card will rest: the panel's top (2000 here) plus the card's place in it, which jsdom lays out at 0.
    expect(scrollBy.mock.calls).toEqual(scrolls ? [[{ top: 2000 - window.innerHeight / 2, behavior: 'instant' }]] : [])
  })

  it('moves the tree up and down with one finger, too', () => {
    layOut({ tree: 600 })
    renderTree('jr')
    const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
    const finger = (clientY: number) => ({ pointerId: 1, pointerType: 'touch', clientX: 100, clientY })
    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toBe('')

    fireEvent.pointerDown(panel, finger(100))
    fireEvent.pointerMove(panel, finger(110))
    fireEvent.pointerMove(panel, finger(140))

    expect(document.querySelector<HTMLElement>('[data-stage]')!.style.transform).toMatch(/, 40px\) scale\(1\)$/)
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

  it.each([
    { name: 'a grandparent', viewer: 'fritz', card: 'Joseph', data: tree, runs: [['up:jr', '0ms', 'down'], ['sib:fritz', '180ms', 'down']] },
    { name: 'a sibling', viewer: 'fritz', card: 'Maya', data: tree, runs: [['sib:maya', '0ms', 'up'], ['sib:fritz', '180ms', 'down']] },
    { name: 'a child', viewer: 'fritz', card: 'Tim', data: tree, runs: [['kids:1', '0ms', 'up']] },
    { name: 'a grandchild', viewer: 'rose', card: 'Naomie', data: fostering, runs: [['down:mt', '0ms', 'up'], ['kids:0', '180ms', 'up']] },
  ])('runs the green from $name to the focus person, one line after another', ({ viewer, card, data, runs }) => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      renderTree(viewer, { data })
      // Once the lines have drawn in.
      act(() => vi.advanceTimersByTime(2000))
    } finally {
      vi.useRealTimers()
    }

    fireEvent.pointerEnter(personOf(card), { pointerType: 'mouse' })

    for (const [line, delay, way] of runs) {
      const path = document.querySelector<SVGElement>(`[data-chain="${line}"]`)!
      expect(path.style.animationDelay).toBe(delay)
      expect(path.classList.contains(styles.chainBack!)).toBe(way === 'up')
    }
  })

  it('draws a green line in with the rest, so a card already under the pointer lights it as it draws', () => {
    renderTree('fritz')

    fireEvent.pointerEnter(personOf('Joseph'), { pointerType: 'mouse' })

    expect(document.querySelector<SVGElement>('[data-chain="up:jr"]')!.style.animationDelay).toBe('480ms')
    expect(document.querySelector<SVGElement>('[data-chain="sib:fritz"]')!.style.animationDelay).toBe(`${560 + 180}ms`)
  })

  it('runs the green afresh when the lift moves to a card sharing a line', () => {
    renderTree('fritz')
    fireEvent.pointerEnter(personOf('Jean-Robert'), { pointerType: 'mouse' })
    const before = document.querySelector('[data-chain="sib:fritz"]')

    fireEvent.pointerLeave(personOf('Jean-Robert'), { pointerType: 'mouse' })
    fireEvent.pointerEnter(personOf('Joseph'), { pointerType: 'mouse' })

    expect(document.querySelector('[data-chain="sib:fritz"]')).not.toBe(before)
  })

  it('fades back everyone off the green while a card is lifted', () => {
    renderTree('fritz')
    const dimmed = () => [...document.querySelectorAll('[data-person]')].filter((p) => p.querySelector(`.${styles.dim}`)).map((p) => p.getAttribute('data-person'))

    fireEvent.pointerEnter(personOf('Joseph'), { pointerType: 'mouse' })
    expect(dimmed()).toContain('tim')
    expect(dimmed()).not.toContain('joseph')
    expect(dimmed()).not.toContain('jr')
    expect(dimmed()).not.toContain('fritz')

    fireEvent.pointerLeave(personOf('Joseph'), { pointerType: 'mouse' })
    expect(dimmed()).toEqual([])

    // A partner shares the household: no green, so nobody fades.
    fireEvent.pointerEnter(personOf('Fabienne'), { pointerType: 'mouse' })
    expect(dimmed()).toEqual([])
  })

  describe('flying the cards to their new places', () => {
    const animate = vi.fn()
    beforeEach(() => {
      animate.mockClear()
      Object.defineProperty(HTMLElement.prototype, 'animate', { value: animate, configurable: true })
    })
    afterEach(() => {
      delete (HTMLElement.prototype as Partial<HTMLElement>).animate
    })
    const flown = () =>
      (animate.mock.contexts as HTMLElement[]).flatMap((el, i) =>
        el.matches('[data-card]') && String(animate.mock.calls[i]![0][0].transform).startsWith('translate(') ? [el.closest('[data-person]')!.getAttribute('data-person')] : [],
      )

    it('flies the cards still in view from where they were, on centering', () => {
      renderTree('fritz')
      expect(animate).not.toHaveBeenCalled()

      fireEvent.click(cardOf('Jean-Robert'))

      expect(flown()).toEqual(expect.arrayContaining(['fritz', 'jr', 'maya', 'joseph']))
      // The lines wait for the cards to land.
      expect(parseInt(document.querySelector<SVGElement>('[data-line="sib:jr"]')!.style.animationDelay)).toBeGreaterThanOrEqual(520)
    })

    it('doesn’t fly them with reduced motion', () => {
      vi.stubGlobal('matchMedia', (query: string) => ({
        matches: query.includes('reduce'),
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {},
      }))
      renderTree('fritz')

      fireEvent.click(cardOf('Jean-Robert'))

      expect(animate).not.toHaveBeenCalled()
    })
  })

  describe('dragging past the limits', () => {
    const dragFar = () => {
      const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
      const mouse = { pointerId: 1, pointerType: 'mouse', button: 0 }
      fireEvent.pointerDown(panel, { ...mouse, clientX: 0, clientY: 0 })
      fireEvent.pointerMove(panel, { ...mouse, clientX: 10, clientY: 0 })
      fireEvent.pointerMove(panel, { ...mouse, clientX: 2000, clientY: 0 })
      return () => fireEvent.pointerUp(window, mouse)
    }
    const shownX = () => Number(document.querySelector<HTMLElement>('[data-stage]')!.style.transform.match(/translate\((-?[\d.]+)px/)![1])

    it('stretches the tree past them, then springs it back to keep 60px in the panel', () => {
      vi.useFakeTimers({ toFake: ['requestAnimationFrame', 'cancelAnimationFrame', 'performance'] })
      try {
        layOut({ tree: 600 })
        renderTree('fritz')

        const letGo = dragFar()
        // 60px of the 190px card stays in: 540, and 35% of the rest of the way beyond.
        expect(shownX()).toBeCloseTo(540 + (2000 - 540) * 0.35, 0)

        letGo()
        act(() => vi.advanceTimersByTime(3000))
        expect(shownX()).toBeCloseTo(540, 0)
      } finally {
        vi.useRealTimers()
      }
    })

    it('holds a tree caught as it springs back, and lets it spring back once let go', () => {
      vi.useFakeTimers({ toFake: ['requestAnimationFrame', 'cancelAnimationFrame', 'performance'] })
      try {
        layOut({ tree: 600 })
        renderTree('fritz')
        dragFar()()
        act(() => vi.advanceTimersByTime(50))
        const caught = shownX()
        expect(caught).toBeGreaterThan(540)

        const panel = screen.getByRole('region', { name: 'Family tree centered on you' })
        fireEvent.pointerDown(panel, { pointerId: 2, pointerType: 'mouse', button: 0, clientX: 0, clientY: 0 })
        act(() => vi.advanceTimersByTime(500))
        expect(shownX()).toBe(caught)

        fireEvent.pointerUp(window, { pointerId: 2 })
        act(() => vi.advanceTimersByTime(3000))
        expect(shownX()).toBeCloseTo(540, 0)
      } finally {
        vi.useRealTimers()
      }
    })

    it('stops at them with reduced motion', () => {
      vi.stubGlobal('matchMedia', (query: string) => ({
        matches: query.includes('reduce'),
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {},
      }))
      layOut({ tree: 600 })
      renderTree('fritz')

      dragFar()

      expect(shownX()).toBe(540)
    })
  })
})
