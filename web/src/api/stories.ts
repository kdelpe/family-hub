import { api } from './client'
import type { DatePrecision } from './events'

export type StoryPerson = { id: string; first_name: string; last_name: string | null }

// recorded_on is "YYYY-MM-DD", to be read with its precision. Lists leave out chapters and transcript.
export type Story = {
  id: string
  title: string
  status: 'draft' | 'published'
  consent_confirmed: boolean
  teller: StoryPerson | null
  recorded_on: string | null
  recorded_on_precision: DatePrecision | null
  place: string | null
  summary: string | null
  language: 'ht' | 'fr' | 'en' | 'mixed' | null
  chapters?: { at: number; title: string }[]
  transcript?: { at: number; text: string }[]
  topics: string[]
  people: StoryPerson[]
  other_people: string[]
  featured: boolean
  created_by: string | null
  created_at: string
}

export const storyKeys = {
  all: ['stories'] as const,
  published: ['stories', 'published'] as const,
  drafts: ['stories', 'drafts'] as const,
  search: (q: string) => ['stories', 'search', q] as const,
  one: (id: string) => ['stories', 'one', id] as const,
}

const list = (query: string, signal?: AbortSignal) =>
  api<{ stories: Story[] }>('GET', `/stories${query}`, { signal }).then((r) => r.stories)

// The story of the week first, then newest first.
export const publishedStoriesQuery = {
  queryKey: storyKeys.published,
  queryFn: ({ signal }: { signal?: AbortSignal }) => list('', signal),
}

// Only the drafts this viewer may see: their own, ones they tell, or everyone's for an admin.
export const draftStoriesQuery = {
  queryKey: storyKeys.drafts,
  queryFn: ({ signal }: { signal?: AbortSignal }) => list('?status=draft', signal),
}

// Searches what was said too, so it asks the server.
export const searchStoriesQuery = (q: string) => ({
  queryKey: storyKeys.search(q),
  queryFn: ({ signal }: { signal?: AbortSignal }) => list(`?${new URLSearchParams({ q })}`, signal),
})

export const storyQuery = (id: string) => ({
  queryKey: storyKeys.one(id),
  queryFn: ({ signal }: { signal?: AbortSignal }) => api<Story>('GET', `/stories/${encodeURIComponent(id)}`, { signal }),
})

// The storyteller's own change: back to a draft, consent withdrawn, so it isn't shared again without asking them.
export const hideStory = (id: string) =>
  api<Story>('PATCH', `/stories/${encodeURIComponent(id)}`, { body: { status: 'draft', consent_confirmed: false } })
