import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { storyKeys, type Story } from '../../api/stories'
import { treeKeys } from '../../api/tree'
import { Stories } from './Stories'

const jr = { id: 'jr', first_name: 'Jean-Robert', last_name: 'Pluviose' }
const nadege = { id: 'nad', first_name: 'Nadège', last_name: 'Pluviose' }

const story = (over: Partial<Story>): Story => ({
  id: 's1',
  title: 'A story',
  status: 'published',
  consent_confirmed: true,
  teller: null,
  recorded_on: null,
  recorded_on_precision: null,
  place: null,
  summary: null,
  language: null,
  topics: [],
  people: [],
  other_people: [],
  featured: false,
  created_by: null,
  created_at: '2026-11-26T10:00:00Z',
  ...over,
})

const published = [
  story({ id: 'brooklyn', title: 'How we came to Brooklyn', teller: jr, featured: true, recorded_on: '2026-10-01', recorded_on_precision: 'month',
    summary: 'The night they left Jacmel.', topics: ['Moving & immigration'] }),
  story({ id: 'grade', title: 'Thirty years of third grade', teller: nadege, recorded_on: '2026-08-01', recorded_on_precision: 'month', topics: ['Work'] }),
  story({ id: 'bus', title: 'The night the band bus broke down', teller: jr, recorded_on: '2009-01-01', recorded_on_precision: 'year', topics: ['Growing up'] }),
]

const person = (id: string, first_name: string, gender: string) => ({ id, first_name, last_name: 'Pluviose', side: 'first', gender, is_deceased: false })
// Maya, Jean-Robert's daughter.
const tree = {
  persons: [person('maya', 'Maya', 'female'), person('jr', 'Jean-Robert', 'male')],
  relationships: [{ id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'jr', child_id: 'maya' }],
}

function renderPage({ stories = published, drafts = [] as Story[] } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: 'maya' })
  client.setQueryData(treeKeys.all, tree)
  client.setQueryData(storyKeys.published, stories)
  client.setQueryData(storyKeys.drafts, drafts)
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={createMemoryRouter([{ path: '/', element: <Stories /> }])} />
    </QueryClientProvider>,
  )
}

const printTitles = (section: HTMLElement) => within(section).queryAllByRole('heading', { level: 3 }).map((h) => h.textContent)
const listSection = () => screen.getAllByRole('region').find((r) => r.querySelector('h2#all-stories'))!

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Stories', () => {
  it('shows the story of the week, its storytellers and every story', () => {
    renderPage()

    expect(screen.getByText('In their own words · 3 stories from 2 storytellers')).toBeInTheDocument()
    const hero = within(screen.getByRole('region', { name: 'Story of the week' }))
    expect(hero.getByRole('heading', { level: 2, name: 'How we came to Brooklyn' })).toBeInTheDocument()
    expect(hero.getByText('Told by Jean-Robert Pluviose')).toBeInTheDocument()
    expect(hero.getByText('Your father · recorded October 2026')).toBeInTheDocument()
    expect(hero.getByRole('link', { name: 'Watch the story' })).toHaveAttribute('href', '/stories/brooklyn')
    expect(screen.getByRole('button', { name: 'Jean-Robert Pluviose, 2 stories' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Nadège Pluviose, 1 story' })).toBeInTheDocument()
    expect(printTitles(listSection())).toEqual(['How we came to Brooklyn', 'Thirty years of third grade', 'The night the band bus broke down'])
    expect(screen.getByText('Jean-Robert Pluviose · 2009')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Thirty years of third grade' })).toHaveAttribute('href', '/stories/grade')
  })

  it('shows only a storyteller’s stories, until they are picked again', () => {
    renderPage()
    const jean = screen.getByRole('button', { name: 'Jean-Robert Pluviose, 2 stories' })

    fireEvent.click(jean)

    expect(jean).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('heading', { level: 2, name: 'Stories told by Jean-Robert Pluviose' })).toBeInTheDocument()
    expect(printTitles(listSection())).toEqual(['How we came to Brooklyn', 'The night the band bus broke down'])

    fireEvent.click(jean)

    expect(printTitles(listSection())).toHaveLength(3)
  })

  it('shows a topic’s stories in the Add a story guide’s order of topics', () => {
    renderPage()
    const topics = within(screen.getByRole('group', { name: 'Topics' }))
    expect(topics.getAllByRole('button').map((b) => b.textContent)).toEqual(['Growing up', 'Moving & immigration', 'Work'])

    fireEvent.click(topics.getByRole('button', { name: 'Work' }))

    expect(screen.getByRole('heading', { level: 2, name: 'Work' })).toBeInTheDocument()
    expect(printTitles(listSection())).toEqual(['Thirty years of third grade'])
  })

  it('asks the server, so what was said is searched too', async () => {
    const fetch = vi.fn(async () => Response.json({ stories: [published[0]] }))
    vi.stubGlobal('fetch', fetch)
    renderPage()

    fireEvent.change(screen.getByRole('searchbox', { name: 'Search stories' }), { target: { value: ' socks ' } })

    await waitFor(() => expect(printTitles(listSection())).toEqual(['How we came to Brooklyn']))
    expect(fetch).toHaveBeenCalledWith('/api/stories?q=socks', expect.anything())
  })

  it('names the words searched when nothing matches them', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Response.json({ stories: [] })))
    renderPage()

    fireEvent.change(screen.getByRole('searchbox', { name: 'Search stories' }), { target: { value: 'zzz' } })

    expect(await screen.findByText('No stories match “zzz”.')).toBeInTheDocument()
  })

  it('says when nothing matches', () => {
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Nadège Pluviose, 1 story' }))
    fireEvent.click(screen.getByRole('button', { name: 'Growing up' }))

    expect(screen.getByText('No stories match. Try another storyteller or topic.')).toBeInTheDocument()
  })

  it('shows the drafts the viewer may see, marked as drafts', () => {
    renderPage({ drafts: [story({ id: 'kitchen', title: 'Grandma’s kitchen in Jacmel', status: 'draft', teller: nadege })] })

    const drafts = screen.getAllByRole('region').find((r) => r.querySelector('h2#drafts'))!
    expect(within(drafts).getByText('only you and admins see these')).toBeInTheDocument()
    expect(printTitles(drafts)).toEqual(['Grandma’s kitchen in Jacmel'])
    expect(within(drafts).getByText('Draft')).toBeInTheDocument()
  })

  it('has an empty page until the first story is shared', () => {
    renderPage({ stories: [] })

    expect(screen.getByRole('heading', { level: 2, name: 'No stories yet' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Story of the week' })).not.toBeInTheDocument()
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
    expect(screen.getByText('In their own words')).toBeInTheDocument()
  })
})
