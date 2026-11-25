import { focusManager, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { domMax, LazyMotion, MotionGlobalConfig } from 'motion/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys } from '../../api/auth'
import { type Contact, contactKeys } from '../../api/contacts'
import { type Person, personKeys } from '../../api/persons'
import type { Tree, TreePerson } from '../../api/tree'
import { PersonProfile } from './PersonProfile'

const slim = (id: string, first_name: string, gender: TreePerson['gender'], is_deceased = false): TreePerson => ({
  id,
  first_name,
  last_name: id === 'lucien' ? 'Monestime' : 'Pluviose',
  side: 'first',
  gender,
  is_deceased,
  bio_excerpt: null,
})

const tree: Tree = {
  persons: [
    slim('lucien', 'Lucien', 'male', true),
    slim('nadege', 'Nadège', 'female'),
    slim('jeanrobert', 'Jean-Robert', 'male'),
    slim('maya', 'Maya', 'female'),
    slim('sabine', 'Sabine', 'female'),
  ],
  relationships: [
    { id: 'r1', type: 'parent_child', subtype: 'biological', parent_id: 'lucien', child_id: 'nadege' },
    { id: 'r2', type: 'couple', subtype: 'married', ended: false, person_ids: ['jeanrobert', 'nadege'] },
    { id: 'r3', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'maya' },
    { id: 'r4', type: 'parent_child', subtype: 'biological', parent_id: 'jeanrobert', child_id: 'maya' },
    { id: 'r5', type: 'parent_child', subtype: 'biological', parent_id: 'jeanrobert', child_id: 'sabine' },
  ],
}

const full = (id: string, over: Partial<Person> = {}): Person => ({
  ...slim(id, tree.persons.find((p) => p.id === id)!.first_name, null),
  maiden_name: null,
  birth_place: null,
  birth_date: null,
  birth_date_precision: null,
  death_date: null,
  death_date_precision: null,
  is_deceased: false,
  bio: null,
  ...over,
})

const nadege = full('nadege', {
  maiden_name: 'Monestime',
  birth_date: '--11-19',
  birth_date_precision: 'day',
  birth_place: 'Brooklyn, New York',
  bio: 'Taught third grade for thirty years.',
})
const everyone = [
  full('lucien', { is_deceased: true, birth_date: '1935', birth_date_precision: 'circa', death_date: '1998', death_date_precision: 'year' }),
  nadege,
  full('jeanrobert'),
  full('maya'),
  full('sabine'),
]

type Answer = { status: number; body?: unknown }

function renderPage({
  role = 'member',
  contacts = [{ kind: 'phone', requestable: true, my_request_status: 'none' }],
  answers = {},
  cached = {},
  motion = false,
}: {
  role?: 'member' | 'admin'
  contacts?: Contact[]
  answers?: Record<string, Answer | Answer[]>
  cached?: Record<string, { person: Person; contacts: Contact[] }>
  // Motion's features loaded and animating, as in the app; otherwise m.* elements render plainly.
  motion?: boolean
} = {}) {
  const all: Record<string, Answer | Answer[]> = {
    'GET /api/persons/nadege': { status: 200, body: nadege },
    'GET /api/tree': { status: 200, body: tree },
    'GET /api/persons/nadege/contacts': { status: 200, body: { contacts } },
    ...answers,
  }
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const ids = url.startsWith('/api/persons?ids=') ? url.slice('/api/persons?ids='.length).split(',') : null
    if (ids && !answers[`GET ${url}`]) {
      const persons = everyone.filter((p) => ids.includes(p.id))
      return Promise.resolve(new Response(JSON.stringify({ persons }), { status: 200 }))
    }
    const queued = all[`${init?.method ?? 'GET'} ${url}`]
    // A list answers in turn, then keeps giving its last answer.
    const answer = Array.isArray(queued) ? (queued.length > 1 ? queued.shift() : queued[0]) : queued
    return answer
      ? Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
      : Promise.reject(new TypeError('Failed to fetch'))
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u', email: 'maya.pluviose@example.com', role, person_id: 'maya' })
  for (const [id, seen] of Object.entries(cached)) {
    client.setQueryData(personKeys.one(id), seen.person)
    client.setQueryData(contactKeys.of(id), seen.contacts)
  }
  const router = createMemoryRouter([{ path: '/people/:id', element: <PersonProfile /> }], {
    initialEntries: ['/people/nadege'],
  })
  const page = (
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  if (motion) {
    MotionGlobalConfig.skipAnimations = false
  }
  render(motion ? <LazyMotion features={domMax}>{page}</LazyMotion> : page)
  return fetch
}

afterEach(() => {
  MotionGlobalConfig.skipAnimations = true
  vi.unstubAllGlobals()
  focusManager.setFocused(undefined)
})

describe('PersonProfile', () => {
  it('asks only for the relatives it shows, never everyone', async () => {
    const fetch = renderPage()

    await screen.findByText('c. 1935 – 1998')

    const asked = fetch.mock.calls.map(([url]) => url).filter((url) => url.startsWith('/api/persons?'))
    expect(asked).toEqual([`/api/persons?ids=${['jeanrobert', 'lucien', 'maya', 'sabine'].join(',')}`])
  })

  it('shows who they are to you, their facts, and their family', async () => {
    renderPage()

    expect(await screen.findByRole('heading', { level: 1, name: 'Nadège Pluviose' })).toBeInTheDocument()
    expect(screen.getByText('Mother')).toBeInTheDocument()
    expect(screen.getByText('née Monestime · Birthday November 19 · Brooklyn, New York')).toBeInTheDocument()
    expect(screen.getByText('Taught third grade for thirty years.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Center the tree on Nadège' })).toHaveAttribute('href', '/tree?person=nadege')

    const family = screen.getByRole('region', { name: 'Nadège’s family' })
    await within(family).findByText('c. 1935 – 1998')
    const group = (title: string) => within(screen.getByRole('heading', { name: title }).parentElement!).getAllByRole('link')
    expect(group('Parents').map((a) => a.textContent)).toEqual(['LMLucien Monestimec. 1935 – 1998'])
    expect(group('Married to').map((a) => a.textContent)).toEqual(['JPJean-Robert PluvioseFather'])
    expect(group('Children').map((a) => a.textContent)).toEqual(['MPMaya PluvioseYou'])
    expect(group('Step-children').map((a) => a.textContent)).toEqual(['SPSabine PluvioseJean-Robert’s daughter'])
    expect(group('Children')[0]).toHaveAttribute('href', '/people/maya')
  })

  it('asks for a phone number, then says it was asked', async () => {
    const fetch = renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'phone', my_request_status: 'requested' } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    expect(await screen.findByText('Phone number requested', { selector: '[role=status]' })).toBeInTheDocument()
    expect(fetch).toHaveBeenCalledWith(
      '/api/persons/nadege/contact-requests',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ kind: 'phone' }) }),
    )
  })

  it('starts fresh on the next profile after asking on this one', async () => {
    const phone: Contact[] = [{ kind: 'phone', requestable: true, my_request_status: 'none' }]
    renderPage({
      // Seen before, so the next profile's buttons render at once instead of after a load.
      cached: { jeanrobert: { person: full('jeanrobert'), contacts: phone } },
      answers: {
        'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'phone', my_request_status: 'requested' } },
        'GET /api/persons/jeanrobert': { status: 200, body: full('jeanrobert') },
        'GET /api/persons/jeanrobert/contacts': { status: 200, body: { contacts: phone } },
      },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))
    await screen.findByText('Phone number requested', { selector: '[role=status]' })
    fireEvent.click(screen.getByRole('link', { name: /Jean-Robert Pluviose/ }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Jean-Robert Pluviose' })).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Request phone number' })).toBeInTheDocument()
    expect(screen.queryByText('Phone number requested')).not.toBeInTheDocument()
  })

  it('asks for contacts afresh on each visit, never showing what an earlier one saw', async () => {
    const sharedPhone: Contact = {
      id: 'c1',
      kind: 'phone',
      platform: null,
      label: null,
      value: '+1 718 555 0123',
      visibility: 'on_request',
      href: null,
      shared_with_you: true,
    }
    renderPage({
      answers: {
        'GET /api/persons/nadege/contacts': [
          { status: 200, body: { contacts: [sharedPhone] } },
          // Nadège took the share back while Maya was on Jean-Robert's profile.
          { status: 200, body: { contacts: [{ kind: 'phone', requestable: true, my_request_status: 'requested' }] } },
        ],
        'GET /api/persons/jeanrobert': { status: 200, body: full('jeanrobert') },
        'GET /api/persons/jeanrobert/contacts': { status: 200, body: { contacts: [] } },
      },
    })
    expect(await screen.findByText('Phone: +1 718 555 0123')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('link', { name: /Jean-Robert Pluviose/ }))
    await screen.findByRole('heading', { level: 1, name: 'Jean-Robert Pluviose' })

    fireEvent.click(await screen.findByRole('link', { name: /Nadège Pluviose/ }))

    expect(screen.getByRole('heading', { level: 1, name: 'Nadège Pluviose' })).toBeInTheDocument()
    expect(screen.queryByText('Phone: +1 718 555 0123')).not.toBeInTheDocument()
    expect(await screen.findByText('Phone number requested', { selector: ':not([role=status])' })).toBeInTheDocument()
    expect(screen.queryByText('Phone: +1 718 555 0123')).not.toBeInTheDocument()
  })

  it('drops the button when the number was already shared', async () => {
    renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 200, body: { kind: 'phone', my_request_status: 'shared' } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    await waitFor(() => expect(screen.queryByRole('button', { name: 'Request phone number' })).not.toBeInTheDocument())
    expect(screen.queryByText('Phone number requested')).not.toBeInTheDocument()
  })

  it.each([
    { name: 'the tree', key: 'GET /api/tree' },
    { name: 'the contacts', key: 'GET /api/persons/nadege/contacts' },
    { name: 'the relatives', key: `GET /api/persons?ids=${['jeanrobert', 'lucien', 'maya', 'sabine'].join(',')}` },
  ])('says when $name can’t load', async ({ key }) => {
    renderPage({ answers: { [key]: { status: 500, body: { error: { code: 'internal', message: 'x' } } } } })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })

  it('says why a login linked to nobody can’t ask', async () => {
    renderPage({
      answers: { 'POST /api/persons/nadege/contact-requests': { status: 403, body: { error: { code: 'forbidden', message: 'x' } } } },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Request phone number' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Your login isn’t linked to you in the tree yet')
  })

  const contactsWith = (sharedWithYou: boolean): Contact[] => [
    {
      id: 'c1',
      kind: 'phone',
      platform: null,
      label: 'Mobile',
      value: '(555) 010-4121',
      visibility: 'on_request',
      href: null,
      shared_with_you: sharedWithYou,
    },
    {
      id: 'c2',
      kind: 'email',
      platform: null,
      label: null,
      value: 'nadege.p@example.com',
      visibility: 'family',
      href: null,
      shared_with_you: false,
    },
  ]

  // An admin gets every row, none marked shared with them.
  it.each([
    { name: 'shared with you', sharedWithYou: true, shown: true },
    { name: 'an admin’s view', sharedWithYou: false, shown: false },
  ])('shows a phone number only when $name', async ({ sharedWithYou, shown }) => {
    renderPage({ role: sharedWithYou ? 'member' : 'admin', contacts: contactsWith(sharedWithYou) })

    await screen.findByRole('heading', { level: 1, name: 'Nadège Pluviose' })
    await screen.findByRole('link', { name: 'Center the tree on Nadège' })
    expect(Boolean(screen.queryByText('Mobile: (555) 010-4121'))).toBe(shown)
    expect(screen.queryByText(/nadege\.p@example\.com/)).not.toBeInTheDocument()
  })

  it('hides a shared phone number once a later refresh fails', async () => {
    renderPage({
      answers: {
        'GET /api/persons/nadege/contacts': [
          { status: 200, body: { contacts: contactsWith(true) } },
          { status: 500, body: { error: { code: 'internal', message: 'x' } } },
        ],
      },
    })
    expect(await screen.findByText('Mobile: (555) 010-4121')).toBeInTheDocument()

    // Coming back to the window refreshes the contacts; this time the server fails.
    act(() => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.queryByText('Mobile: (555) 010-4121')).not.toBeInTheDocument()
  })

  it('hides a shared phone number while a refresh is on its way', async () => {
    const fetch = renderPage({ contacts: contactsWith(true) })
    expect(await screen.findByText('Mobile: (555) 010-4121')).toBeInTheDocument()
    const answer = fetch.getMockImplementation()!
    let respond: (value: Response) => void = () => {}
    fetch.mockImplementation((url, init) =>
      url === '/api/persons/nadege/contacts' ? new Promise((resolve) => (respond = resolve)) : answer(url, init),
    )

    act(() => {
      focusManager.setFocused(false)
      focusManager.setFocused(true)
    })

    await waitFor(() => expect(screen.queryByText('Mobile: (555) 010-4121')).not.toBeInTheDocument())
    act(() => respond(Response.json({ contacts: contactsWith(true) })))
    expect(await screen.findByText('Mobile: (555) 010-4121')).toBeInTheDocument()
  })

  it('says when there’s no such person', async () => {
    renderPage({ answers: { 'GET /api/persons/nadege': { status: 404, body: { error: { code: 'not_found', message: 'x' } } } } })

    expect(await screen.findByText(/We couldn’t find that person/)).toBeInTheDocument()
    expect(document.title).toBe('Not found — Pluviose Family Hub')
    expect(screen.getByRole('link', { name: 'See everyone in Members' })).toHaveAttribute('href', '/members')
  })
})

describe('PersonProfile adding a relative', () => {
  const pick = (name: string | RegExp) => fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name }))
  const type = (text: string) => fireEvent.change(within(screen.getByRole('dialog')).getByRole('searchbox', { name: 'Name' }), { target: { value: text } })
  const open = async (label: string) => fireEvent.click(await screen.findByRole('button', { name: label }))
  const paolo = { ...full('nadege'), id: 'paolo', first_name: 'Paolo', last_name: 'Pluviose' }
  const added = (person: Person, ids: string[]) => ({ status: 201, body: { person, relationships: ids.map((id) => ({ id })) } })

  it.each([
    { role: 'member' as const, want: [] },
    { role: 'admin' as const, want: [['Parents', 'Add a parent'], ['Married to', 'Add a partner'], ['Siblings', 'Add a brother or sister'], ['Children', 'Add a child']] },
  ])('shows the Add cards, in their groups, to $role only', async ({ role, want }) => {
    renderPage({ role })
    await screen.findByRole('heading', { name: 'Parents' })

    const cards = screen.queryAllByRole('button', { name: /^Add a/ })
    expect(cards.map((b) => [b.closest('div')!.querySelector('h3')!.textContent, b.textContent!.replace('+', '')])).toEqual(want)
  })

  it.each([
    {
      name: 'a new child, with their other parent',
      open: 'Add a child',
      steps: () => {
        type('Paolo  Pluviose')
        pick(/^Add “Paolo Pluviose” as someone new/)
        pick('Born to Nadège')
        pick(/^Jean-Robert Pluviose/)
      },
      sentence: 'Paolo Pluviose will be Nadège’s child. Jean-Robert is the other parent.',
      body: { relation: 'child', person: { first_name: 'Paolo', last_name: 'Pluviose' }, subtype: 'biological', other_parent_id: 'jeanrobert' },
    },
    {
      name: 'a former partner already in the tree',
      open: 'Add a partner',
      steps: () => {
        type('sab')
        pick(/^Sabine Pluviose/)
        pick('No, partners')
        pick(/^No, it ended/)
      },
      sentence: 'Sabine Pluviose will be Nadège’s former partner.',
      body: { relation: 'partner', person_id: 'sabine', subtype: 'partner', ended: true },
    },
    {
      name: 'an adoptive mother',
      open: 'Add a parent',
      steps: () => {
        type('Sabine')
        pick(/^Sabine Pluviose/)
        pick('Adopted Nadège')
      },
      sentence: 'Sabine Pluviose will be Nadège’s adoptive mother.',
      body: { relation: 'parent', person_id: 'sabine', subtype: 'adoptive' },
    },
    {
      name: 'a new sister, through the one parent recorded',
      open: 'Add a brother or sister',
      steps: () => {
        type('Odette')
        pick(/^Add “Odette” as someone new/)
      },
      sentence: 'Odette will be Nadège’s sibling. They’ll be linked as a child of Lucien.',
      body: { relation: 'sibling', person: { first_name: 'Odette', last_name: null } },
    },
  ])('adds $name', async ({ open: label, steps, sentence, body }) => {
    const fetch = renderPage({ role: 'admin', answers: { 'POST /api/persons/nadege/relatives': added(paolo, ['n1']) } })
    await open(label)

    steps()

    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText(sentence)).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: /^Add / }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(fetch).toHaveBeenCalledWith('/api/persons/nadege/relatives', expect.objectContaining({ method: 'POST', body: JSON.stringify(body) }))
  })

  it('shows the new relative, and Undo takes off the links and the person added', async () => {
    const withPaolo: Tree = {
      persons: [...tree.persons, slim('paolo', 'Paolo', 'male')],
      relationships: [...tree.relationships, { id: 'n1', type: 'parent_child', subtype: 'biological', parent_id: 'nadege', child_id: 'paolo' }],
    }
    const fetch = renderPage({
      role: 'admin',
      answers: {
        'GET /api/tree': [
          { status: 200, body: tree },
          { status: 200, body: withPaolo },
          { status: 200, body: tree },
        ],
        'POST /api/persons/nadege/relatives': added(paolo, ['n1']),
        'DELETE /api/relationships/n1': { status: 204 },
        'DELETE /api/persons/paolo': { status: 204 },
      },
    })
    await open('Add a child')
    type('Paolo Pluviose')
    pick(/^Add “Paolo Pluviose” as someone new/)
    pick('Born to Nadège')
    pick(/^Someone else/)
    pick('Add Paolo')

    expect(await screen.findByText('Paolo added as Nadège’s child.')).toBeInTheDocument()
    const card = await screen.findByRole('link', { name: /Paolo Pluviose/ })
    expect(card.className).toMatch(/fresh/)
    expect(screen.getByRole('button', { name: 'Add a child' })).toHaveFocus()

    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))

    await waitFor(() => expect(screen.queryByText('Paolo added as Nadège’s child.')).not.toBeInTheDocument())
    const deletes = fetch.mock.calls.filter(([, init]) => init?.method === 'DELETE').map(([url]) => url)
    expect(deletes).toEqual(['/api/relationships/n1', '/api/persons/paolo'])
    await waitFor(() => expect(screen.queryByRole('link', { name: /Paolo Pluviose/ })).not.toBeInTheDocument())
  })

  it('keeps someone already in the tree when undone', async () => {
    const fetch = renderPage({
      role: 'admin',
      answers: { 'POST /api/persons/nadege/relatives': added(full('sabine'), ['n1']), 'DELETE /api/relationships/n1': { status: 204 } },
    })
    await open('Add a parent')
    type('Sabine')
    pick(/^Sabine Pluviose/)
    pick('Adopted Nadège')
    pick('Add Sabine')
    fireEvent.click(await screen.findByRole('button', { name: 'Undo' }))

    await waitFor(() => expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument())
    expect(fetch.mock.calls.filter(([, init]) => init?.method === 'DELETE').map(([url]) => url)).toEqual(['/api/relationships/n1'])
  })

  it('carries on from where a failed Undo stopped', async () => {
    const fetch = renderPage({
      role: 'admin',
      answers: {
        'POST /api/persons/nadege/relatives': added(paolo, ['n1', 'n2']),
        'DELETE /api/relationships/n1': [{ status: 204 }, { status: 404, body: { error: { code: 'not_found', message: 'not found' } } }],
        'DELETE /api/relationships/n2': [{ status: 503, body: { error: { code: 'busy', message: 'busy' } } }, { status: 204 }],
        'DELETE /api/persons/paolo': { status: 204 },
      },
    })
    await open('Add a child')
    type('Paolo Pluviose')
    pick(/^Add “Paolo Pluviose” as someone new/)
    pick('Born to Nadège')
    pick(/^Jean-Robert Pluviose/)
    pick('Add Paolo')

    fireEvent.click(await screen.findByRole('button', { name: 'Undo' }))
    expect(await screen.findByText('The family hub is busy right now. Try again in a moment.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))

    await waitFor(() => expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument())
    expect(fetch.mock.calls.filter(([, init]) => init?.method === 'DELETE').map(([url]) => url)).toEqual([
      '/api/relationships/n1',
      '/api/relationships/n2',
      '/api/relationships/n1',
      '/api/relationships/n2',
      '/api/persons/paolo',
    ])
  })

  it('offers “Both” only to someone with two parents', async () => {
    const threeParents: Tree = {
      persons: [...tree.persons, slim('rose', 'Rose', 'female'), slim('marc', 'Marc', 'male')],
      relationships: [
        ...tree.relationships,
        { id: 'p2', type: 'parent_child', subtype: 'biological', parent_id: 'rose', child_id: 'nadege' },
        { id: 'p3', type: 'parent_child', subtype: 'adoptive', parent_id: 'marc', child_id: 'nadege' },
      ],
    }
    renderPage({ role: 'admin', answers: { 'GET /api/tree': { status: 200, body: threeParents } } })
    await open('Add a brother or sister')
    type('Odette')
    pick(/^Add “Odette” as someone new/)

    const options = within(screen.getByRole('dialog')).getAllByRole('button', { name: /^(Both|Only)/ })
    expect(options.map((b) => b.textContent)).toEqual([
      'LMOnly Lucien Monestime',
      'RPOnly Rose Pluviose',
      'MPOnly Marc Pluviose',
    ])
  })

  it('links an existing half sibling only through the parent they lack', async () => {
    const twoParents: Tree = {
      persons: [...tree.persons, slim('rose', 'Rose', 'female'), slim('ti', 'Ti', 'female')],
      relationships: [
        ...tree.relationships,
        { id: 'p2', type: 'parent_child', subtype: 'biological', parent_id: 'rose', child_id: 'nadege' },
        { id: 'p3', type: 'parent_child', subtype: 'biological', parent_id: 'rose', child_id: 'ti' },
      ],
    }
    renderPage({ role: 'admin', answers: { 'GET /api/tree': { status: 200, body: twoParents } } })
    await open('Add a brother or sister')
    type('Ti')
    pick(/^Ti Pluviose/)

    expect(within(screen.getByRole('dialog')).getByRole('button', { name: /^Only Rose Pluviose/ })).toBeDisabled()
    pick(/^Both: Lucien and Rose/)

    expect(screen.getByText('Ti Pluviose will be Nadège’s sister. They’ll be linked as a child of Lucien.')).toBeInTheDocument()
  })

  it.each([
    { name: 'a half-sister, each with a parent the other lacks', who: 'Ti', only: /^Only Rose Pluviose/, sentence: 'Ti Pluviose will be Nadège’s half-sister. They’ll be linked as a child of Rose.' },
    {
      name: 'the neutral word, when the parents recorded prove neither',
      who: 'Jean-Robert',
      only: /^Only Lucien Monestime/,
      sentence: 'Jean-Robert Pluviose will be Nadège’s sibling. They’ll be linked as a child of Lucien.',
    },
  ])('calls an existing sibling $name', async ({ who, only, sentence }) => {
    const twoParents: Tree = {
      persons: [...tree.persons, slim('rose', 'Rose', 'female'), slim('zoe', 'Zoé', 'female'), slim('ti', 'Ti', 'female')],
      relationships: [
        ...tree.relationships,
        { id: 'p2', type: 'parent_child', subtype: 'biological', parent_id: 'rose', child_id: 'nadege' },
        { id: 'p3', type: 'parent_child', subtype: 'biological', parent_id: 'zoe', child_id: 'ti' },
      ],
    }
    renderPage({ role: 'admin', answers: { 'GET /api/tree': { status: 200, body: twoParents } } })
    await open('Add a brother or sister')
    type(who)
    pick(new RegExp(`^${who} Pluviose`))
    pick(only)

    expect(screen.getByText(sentence)).toBeInTheDocument()
  })

  it('leaves out an other parent the child already has', async () => {
    // Sabine is Jean-Robert's daughter; Nadège adopts her, with Jean-Robert as the other parent.
    const fetch = renderPage({ role: 'admin', answers: { 'POST /api/persons/nadege/relatives': added(full('sabine'), ['n1']) } })
    await open('Add a child')
    type('Sabine')
    pick(/^Sabine Pluviose/)
    pick('Adopted by Nadège')
    pick(/^Jean-Robert Pluviose/)
    pick('Add Sabine')

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    const body = { relation: 'child', person_id: 'sabine', subtype: 'adoptive' }
    expect(fetch).toHaveBeenCalledWith('/api/persons/nadege/relatives', expect.objectContaining({ body: JSON.stringify(body) }))
  })

  it('keeps Tab inside the sheet', async () => {
    renderPage({ role: 'admin' })
    await open('Add a parent')
    const dialog = screen.getByRole('dialog')
    const cancel = within(dialog).getByRole('button', { name: 'Cancel' })
    const search = within(dialog).getByRole('searchbox', { name: 'Name' })

    // Cancel is the first stop on this screen (Back is hidden), and the search box the last.
    search.focus()
    fireEvent.keyDown(search, { key: 'Tab' })
    expect(cancel).toHaveFocus()
    fireEvent.keyDown(cancel, { key: 'Tab', shiftKey: true })
    expect(search).toHaveFocus()
  })

  it.each([
    // Steps of 10px every 30ms are slow, about 330px/s; 30px every 16ms is a flick, about 1,900px/s.
    { name: 'a long slow drag closes it, giving focus back to its card', by: 200, step: 10, every: 30, closes: true },
    { name: 'a short downward flick closes it', by: 60, step: 30, every: 16, closes: true },
    { name: 'a short slow drag leaves it open', by: 60, step: 10, every: 30, closes: false },
  ])('drags the sheet by its handle: $name', async ({ by, step, every, closes }) => {
    renderPage({ role: 'admin', motion: true })
    await open('Add a parent')
    const grab = document.querySelector<HTMLElement>('[class*="grab"]')!
    const at = (y: number) => ({ pointerId: 1, isPrimary: true, button: 0, buttons: 1, clientX: 100, clientY: y, pageX: 100, pageY: y })

    fireEvent.pointerDown(grab, at(600))
    for (let y = 600 + step; y <= 600 + by; y += step) {
      fireEvent.pointerMove(window, at(y))
      await act(() => new Promise((r) => setTimeout(r, every)))
    }
    fireEvent.pointerUp(window, at(600 + by))

    if (closes) {
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull(), { timeout: 3000 })
      await waitFor(() => expect(screen.getByRole('button', { name: 'Add a parent' })).toHaveFocus())
    } else {
      await act(() => new Promise((r) => setTimeout(r, 700)))
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    }
  })

  it('stays open when dragged down while it saves, the answer still to show', async () => {
    const fetch = renderPage({ role: 'admin', motion: true })
    const answered = fetch.getMockImplementation()!
    fetch.mockImplementation((url: string, init?: RequestInit) =>
      init?.method === 'POST' ? new Promise<Response>(() => {}) : answered(url, init),
    )
    await open('Add a child')
    type('Sabine')
    pick(/^Sabine Pluviose/)
    pick('Adopted by Nadège')
    pick(/^Jean-Robert Pluviose/)
    pick('Add Sabine')
    expect(await within(screen.getByRole('dialog')).findByRole('button', { name: 'Adding…' })).toBeDisabled()
    const grab = document.querySelector<HTMLElement>('[class*="grab"]')!
    const at = (y: number) => ({ pointerId: 1, isPrimary: true, button: 0, buttons: 1, clientX: 100, clientY: y, pageX: 100, pageY: y })

    fireEvent.pointerDown(grab, at(600))
    for (let y = 610; y <= 800; y += 10) {
      fireEvent.pointerMove(window, at(y))
      await act(() => new Promise((r) => setTimeout(r, 30)))
    }
    fireEvent.pointerUp(window, at(800))
    await act(() => new Promise((r) => setTimeout(r, 700)))

    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('clears a filled search box on Escape before closing', async () => {
    renderPage({ role: 'admin' })
    await open('Add a parent')
    type('Sab')
    const search = within(screen.getByRole('dialog')).getByRole('searchbox', { name: 'Name' })

    fireEvent.keyDown(search, { key: 'Escape' })
    expect(search).toHaveValue('')
    fireEvent.keyDown(search, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('says why the server refused, and stays open to go back', async () => {
    renderPage({
      role: 'admin',
      answers: {
        'POST /api/persons/nadege/relatives': {
          status: 409,
          body: { error: { code: 'conflict', message: 'relationship conflict: the child already has two biological parents' } },
        },
      },
    })
    await open('Add a parent')
    type('Sabine')
    pick(/^Sabine Pluviose/)
    pick('Birth mother')
    pick('Add Sabine')

    expect(await within(screen.getByRole('dialog')).findByRole('alert')).toHaveTextContent('The child already has two biological parents.')
    pick('← Back')
    expect(screen.getByRole('heading', { name: 'How is Sabine Nadège’s parent?' })).toHaveFocus()
  })

  it('can’t pick someone already related that way', async () => {
    renderPage({ role: 'admin' })
    await open('Add a child')

    type('Maya')

    expect(within(screen.getByRole('dialog')).getByRole('button', { name: /^Maya Pluviose/ })).toBeDisabled()
    expect(screen.getByText('Already Nadège’s child')).toBeInTheDocument()
  })

  it('asks for a parent before a sibling, and Cancel or Escape closes', async () => {
    const noParents: Tree = { ...tree, relationships: tree.relationships.filter((r) => r.id !== 'r1') }
    renderPage({ role: 'admin', answers: { 'GET /api/tree': { status: 200, body: noParents } } })
    await open('Add a brother or sister')

    expect(screen.getByRole('heading', { name: 'Who are Nadège’s parents?' })).toBeInTheDocument()
    pick(/^Add a parent for Nadège/)
    expect(screen.getByRole('heading', { name: 'Who is Nadège’s parent?' })).toBeInTheDocument()

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await open('Add a parent')
    pick('Cancel')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await waitFor(() => expect(screen.getByRole('button', { name: 'Add a parent' })).toHaveFocus())
  })
})
