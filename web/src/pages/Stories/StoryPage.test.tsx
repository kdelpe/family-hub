import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { storyKeys, storyQuery, type Story } from '../../api/stories'
import { treeKeys } from '../../api/tree'
import { StoryPage } from './StoryPage'

const jr = { id: 'jr', first_name: 'Jean-Robert', last_name: 'Pluviose' }

const story = (over: Partial<Story>): Story => ({
  id: 'brooklyn',
  title: 'How we came to Brooklyn',
  status: 'published',
  consent_confirmed: true,
  teller: jr,
  recorded_on: '2026-10-01',
  recorded_on_precision: 'month',
  place: 'Nadège’s kitchen',
  summary: null,
  language: 'mixed',
  chapters: [
    { at: 0, title: 'Growing up in Jacmel' },
    { at: 1050, title: 'Landing at JFK' },
  ],
  transcript: [
    { at: 0, text: 'We lived two streets from the market in Jacmel.' },
    { at: 62, text: 'We had the sea in front and the mountains behind.' },
    { at: 1050, text: 'When the doors opened at JFK, the cold hit me in the chest.' },
    { at: 1620, text: 'I saw snow for the first time and ran out in my socks.' },
  ],
  topics: [],
  people: [{ id: 'jo', first_name: 'Joseph', last_name: 'Pluviose' }],
  other_people: ['Père Antoine'],
  featured: true,
  created_by: null,
  created_at: '2026-11-26T10:00:00Z',
  ...over,
})

const person = (id: string, first_name: string, gender: string) => ({ id, first_name, last_name: 'Pluviose', side: 'first', gender, is_deceased: false })
const tree = {
  persons: [person('maya', 'Maya', 'female'), person('jr', 'Jean-Robert', 'male')],
  relationships: [{ id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'jr', child_id: 'maya' }],
}
const bus = story({ id: 'bus', title: 'The night the band bus broke down', recorded_on: '2026-09-01', featured: false, chapters: undefined, transcript: undefined })

function renderPage(shown?: Story, published: Story[] = [story({}), bus], personId = 'maya') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'member', person_id: personId })
  client.setQueryData(treeKeys.all, tree)
  client.setQueryData(storyKeys.published, published)
  if (shown) {
    client.setQueryData(storyQuery(shown.id).queryKey, shown)
  }
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={createMemoryRouter([{ path: '/stories/:id', element: <StoryPage /> }], { initialEntries: [`/stories/${shown?.id ?? 'gone'}`] })} />
    </QueryClientProvider>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('StoryPage', () => {
  it('shows the story with its transcript under its chapters, and a player waiting for the video', () => {
    renderPage(story({}))

    expect(screen.getByRole('heading', { level: 1, name: 'How we came to Brooklyn' })).toBeInTheDocument()
    expect(screen.getByText('Recorded October 2026 in Nadège’s kitchen')).toBeInTheDocument()
    expect(screen.getByText('Video coming with uploads')).toBeInTheDocument()
    const transcript = within(screen.getByRole('region', { name: 'Transcript' }))
    expect(transcript.getByText('Spoken in more than one language')).toBeInTheDocument()
    expect(transcript.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['Growing up in Jacmel', 'Landing at JFK'])
    expect(transcript.getByText('17:30')).toBeInTheDocument()
    expect(within(screen.getByRole('region', { name: 'Chapters' })).getByRole('link', { name: /Landing at JFK/ })).toHaveAttribute('href', '#chapter-1')
  })

  it('says who told it, who is in it and what else they told', () => {
    renderPage(story({}))

    const told = within(screen.getByRole('region', { name: 'Told by' }))
    expect(told.getByRole('link', { name: /Jean-Robert Pluviose/ })).toHaveAttribute('href', '/people/jr')
    expect(told.getByText('Your father · 2 stories')).toBeInTheDocument()
    const people = within(screen.getByRole('region', { name: 'People in this story' }))
    expect(people.getByRole('link', { name: 'Joseph Pluviose' })).toHaveAttribute('href', '/people/jo')
    expect(people.getByText('Père Antoine (not in the tree)')).toBeInTheDocument()
    const more = within(screen.getByRole('region', { name: 'More from Jean-Robert' }))
    expect(more.getByRole('link', { name: /The night the band bus broke down/ })).toHaveAttribute('href', '/stories/bus')
  })

  it('searches what was said, marking the words found', () => {
    renderPage(story({}))

    fireEvent.change(screen.getByRole('searchbox', { name: 'Search the transcript' }), { target: { value: 'SNOW' } })

    const transcript = screen.getByRole('region', { name: 'Transcript' })
    expect(within(transcript).getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['Landing at JFK'])
    expect(transcript.querySelectorAll('mark')).toHaveLength(1)
    expect(transcript.querySelector('mark')).toHaveTextContent('snow')
    expect(transcript.querySelectorAll('p')).toHaveLength(2)

    fireEvent.change(screen.getByRole('searchbox', { name: 'Search the transcript' }), { target: { value: 'zzz' } })

    expect(within(transcript).getByText('Nothing matches “zzz”.')).toBeInTheDocument()
  })

  it('marks a draft and leaves out what it doesn’t have', () => {
    renderPage(story({ status: 'draft', transcript: [], chapters: [], people: [], other_people: [], place: null, language: null }))

    expect(screen.getByText('Draft · not shared with the family yet')).toBeInTheDocument()
    expect(screen.getByText('Recorded October 2026')).toBeInTheDocument()
    expect(screen.getByText('No transcript yet.')).toBeInTheDocument()
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Chapters' })).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'People in this story' })).not.toBeInTheDocument()
  })

  it('lets its storyteller hide it, withdrawing their consent', async () => {
    const fetch = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(async () => Response.json(story({ status: 'draft', consent_confirmed: false })))
    vi.stubGlobal('fetch', fetch)
    renderPage(story({}), undefined, 'jr')

    fireEvent.click(screen.getByRole('button', { name: 'Hide this story' }))
    fireEvent.click(within(screen.getByRole('dialog', { name: 'Hide “How we came to Brooklyn”?' })).getByRole('button', { name: 'Hide it' }))

    expect(await screen.findByText('Draft · not shared with the family yet')).toBeInTheDocument()
    const [url, init] = fetch.mock.calls.find(([, init]) => init?.method === 'PATCH')!
    expect(url).toBe('/api/stories/brooklyn')
    expect(JSON.parse(init!.body as string)).toEqual({ status: 'draft', consent_confirmed: false })
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Hide this story' })).not.toBeInTheDocument())
  })

  it('offers hiding to no one but the storyteller', () => {
    renderPage(story({}))

    expect(screen.queryByRole('button', { name: 'Hide this story' })).not.toBeInTheDocument()
  })

  it('jumps from a chapter to its heading, clearing a search that hid it', () => {
    Element.prototype.scrollIntoView = vi.fn()
    renderPage(story({}))
    const search = screen.getByRole('searchbox', { name: 'Search the transcript' })
    fireEvent.change(search, { target: { value: 'snow' } })
    expect(document.getElementById('chapter-0')).toBeNull()

    fireEvent.click(within(screen.getByRole('region', { name: 'Chapters' })).getByRole('link', { name: /Growing up in Jacmel/ }))

    expect(search).toHaveValue('')
    expect(document.getElementById('chapter-0')).toHaveTextContent('Growing up in Jacmel')
    expect(Element.prototype.scrollIntoView).toHaveBeenCalled()
  })

  it('says so when the story isn’t there', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Response.json({ error: { code: 'not_found', message: 'story not found' } }, { status: 404 })))
    renderPage()

    expect(await screen.findByText('This story isn’t here. It may have been removed, or not shared yet.')).toBeInTheDocument()
  })
})
