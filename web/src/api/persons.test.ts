import { afterEach, expect, it, vi } from 'vitest'

import { getPersons } from './persons'

afterEach(() => {
  vi.unstubAllGlobals()
})

it('asks for at most 100 people at a time and joins the answers', async () => {
  const ids = Array.from({ length: 150 }, (_, i) => `p${String(i).padStart(3, '0')}`)
  const fetch = vi.fn((url: string) => {
    const asked = new URL(url, 'http://family.example.com').searchParams.get('ids')!.split(',')
    return Promise.resolve(Response.json({ persons: asked.map((id) => ({ id })) }))
  })
  vi.stubGlobal('fetch', fetch)

  const people = await getPersons(ids)

  expect(fetch).toHaveBeenCalledTimes(2)
  expect(fetch.mock.calls.map(([url]) => new URL(url, 'http://family.example.com').searchParams.get('ids')!.split(',').length)).toEqual([100, 50])
  expect(people.map((p) => p.id)).toEqual(ids)
})
