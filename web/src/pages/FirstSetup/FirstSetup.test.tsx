import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { FirstSetup } from './FirstSetup'

type Answer = { status: number; body?: unknown }

const darlineLogin = { id: 'u', email: 'darline@example.com', role: 'member', person_id: 'darline' }
const darline = { id: 'darline', first_name: 'Darline', last_name: 'Pluviose', bio: 'Physical therapist in Miami.', birth_date: null, birth_date_precision: null }

const contact = (id: string, kind: string, value: string, platform: string | null = null) => ({
  id,
  kind,
  platform,
  label: null,
  value,
  visibility: 'hidden',
  href: null,
  shared_with_you: false,
})

const person = (id: string, first_name: string, last_name: string, side: string | null = 'first', is_deceased = false) => ({
  id,
  first_name,
  last_name,
  side,
  gender: 'unknown',
  is_deceased,
})
const parent = (parent_id: string, child_id: string, subtype = 'biological') => ({ id: `${parent_id}-${child_id}`, type: 'parent_child', subtype, parent_id, child_id })

const tree = {
  persons: [
    person('lucien', 'Lucien', 'Pluviose'),
    person('rose', 'Rose', 'Pluviose'),
    person('frantz', 'Frantz', 'Pluviose'),
    person('mirlande', 'Mirlande', 'Pluviose'),
    person('darline', 'Darline', 'Pluviose'),
    person('paul', 'Paul', 'Pluviose'),
    person('maya', 'Maya', 'Pluviose'),
    person('fritz', 'Fritz', 'Pluviose'),
    person('ghost', 'Ghost', 'Pluviose', 'first', true),
    person('foster', 'Foster', 'Parent'),
  ],
  relationships: [
    parent('lucien', 'frantz'),
    parent('rose', 'frantz'),
    parent('frantz', 'darline'),
    parent('mirlande', 'darline'),
    parent('foster', 'darline', 'foster'),
    parent('lucien', 'paul'),
    parent('rose', 'paul'),
    parent('paul', 'maya'),
    parent('paul', 'fritz'),
    parent('paul', 'ghost'),
  ],
}

function renderPage(over: Record<string, Answer> = {}) {
  const answers: Record<string, Answer> = {
    'GET /api/auth/me': { status: 200, body: darlineLogin },
    'GET /api/persons/darline': { status: 200, body: darline },
    'GET /api/me/contacts': {
      status: 200,
      body: { person_id: 'darline', contacts: [contact('ig', 'social', '@darline.m', 'instagram'), contact('mobile', 'phone', '555-010-6624'), contact('mail', 'email', 'darline@example.com')] },
    },
    'GET /api/tree': { status: 200, body: tree },
    ...over,
  }
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const answer = answers[`${init?.method ?? 'GET'} ${url}`]
    return answer
      ? Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
      : Promise.reject(new TypeError('Failed to fetch'))
  })
  vi.stubGlobal('fetch', fetch)
  const router = createMemoryRouter(
    [
      { path: '/setup', element: <FirstSetup /> },
      { path: '/', element: <p>Home page</p> },
      { path: '/auth/login', element: <p>Sign in page</p> },
    ],
    { initialEntries: ['/setup'] },
  )
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { answers, fetch, client }
}

const click = (name: string) => fireEvent.click(screen.getByRole('button', { name }))

const sent = (fetch: ReturnType<typeof renderPage>['fetch'], method: string, url: string) =>
  fetch.mock.calls.filter(([u, init]) => u === url && init?.method === method).map(([, init]) => JSON.parse(String(init?.body)))

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('FirstSetup', () => {
  it('walks through the three steps and back, saving nothing unchanged', async () => {
    const { fetch } = renderPage()
    expect(await screen.findByLabelText('A line or two about you')).toHaveValue('Physical therapist in Miami.')
    expect(screen.getByText('Step 1 of 3')).toBeInTheDocument()
    expect(screen.getByText('DP')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Back' })).not.toBeInTheDocument()

    click('Continue')
    expect(screen.getByRole('heading', { name: 'How can family reach you?' })).toHaveFocus()
    expect(screen.getByText('Step 2 of 3')).toBeInTheDocument()

    click('Skip for now')
    expect(screen.getByRole('heading', { name: 'Here’s your place in the family' })).toHaveFocus()
    expect(screen.queryByRole('button', { name: 'Skip for now' })).not.toBeInTheDocument()

    click('Back')
    expect(screen.getByRole('heading', { name: 'How can family reach you?' })).toHaveFocus()
    expect(sent(fetch, 'PATCH', '/api/persons/darline')).toEqual([])
  })

  it('saves a changed story on Continue', async () => {
    const { fetch } = renderPage({ 'PATCH /api/persons/darline': { status: 200, body: { ...darline, bio: 'Always bringing the pikliz.' } } })
    fireEvent.change(await screen.findByLabelText('A line or two about you'), { target: { value: ' Always bringing the pikliz. ' } })

    click('Continue')

    expect(await screen.findByRole('heading', { name: 'How can family reach you?' })).toHaveFocus()
    expect(sent(fetch, 'PATCH', '/api/persons/darline')).toEqual([{ bio: 'Always bringing the pikliz.' }])
  })

  it('follows a story refreshed meanwhile, and doesn’t write the old one back', async () => {
    const { answers, fetch, client } = renderPage()
    await screen.findByLabelText('A line or two about you')
    answers['GET /api/persons/darline'] = { status: 200, body: { ...darline, bio: 'Moved to Orlando.' } }

    await act(() => client.invalidateQueries({ queryKey: ['persons', 'darline'] }))

    await vi.waitFor(() => expect(screen.getByLabelText('A line or two about you')).toHaveValue('Moved to Orlando.'))
    click('Continue')
    expect(screen.getByRole('heading', { name: 'How can family reach you?' })).toBeInTheDocument()
    expect(sent(fetch, 'PATCH', '/api/persons/darline')).toEqual([])
  })

  it('stays on the story, saying why, when it can’t be saved', async () => {
    renderPage({ 'PATCH /api/persons/darline': { status: 400, body: { error: { code: 'validation_error', message: 'invalid person: bio is too long' } } } })
    fireEvent.change(await screen.findByLabelText('A line or two about you'), { target: { value: 'Too long' } })

    click('Continue')

    expect(await screen.findByRole('alert')).toHaveTextContent('Bio is too long.')
    expect(screen.getByText('Step 1 of 3')).toBeInTheDocument()
  })

  it('clears a failed save’s message when you skip the story', async () => {
    renderPage({ 'PATCH /api/persons/darline': { status: 400, body: { error: { code: 'validation_error', message: 'invalid person: bio is too long' } } } })
    fireEvent.change(await screen.findByLabelText('A line or two about you'), { target: { value: 'Too long' } })
    click('Continue')
    await screen.findByRole('alert')

    click('Skip for now')
    click('Back')

    expect(screen.getByText('Step 1 of 3')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('skips the story without saving it', async () => {
    const { fetch } = renderPage()
    fireEvent.change(await screen.findByLabelText('A line or two about you'), { target: { value: 'Not yet' } })

    click('Skip for now')

    expect(screen.getByRole('heading', { name: 'How can family reach you?' })).toBeInTheDocument()
    expect(sent(fetch, 'PATCH', '/api/persons/darline')).toEqual([])
  })

  it.each([
    { name: 'Phone', id: 'mobile', visibility: 'on_request', on: 'Family can ask' },
    { name: 'Email', id: 'mail', visibility: 'family', on: 'Visible to family' },
    { name: 'Instagram', id: 'ig', visibility: 'family', on: 'Visible to family' },
  ])('lists your $name as Only me and turns it on as "$on"', async ({ name, id, visibility, on }) => {
    const rows = { mobile: contact('mobile', 'phone', '555-010-6624'), mail: contact('mail', 'email', 'darline@example.com'), ig: contact('ig', 'social', '@darline.m', 'instagram') }
    const saved = { ...rows[id as keyof typeof rows], visibility }
    const { answers, fetch } = renderPage({ [`PATCH /api/me/contacts/${id}`]: { status: 200, body: saved } })
    await screen.findByLabelText('A line or two about you')
    click('Continue')
    const toggle = await screen.findByRole('switch', { name })
    expect(toggle).not.toBeChecked()
    expect(toggle).toHaveAccessibleDescription('Only me')

    // As the server lists it once saved.
    answers['GET /api/me/contacts'] = { status: 200, body: { person_id: 'darline', contacts: Object.values(rows).map((r) => (r.id === id ? saved : r)) } }
    fireEvent.click(toggle)

    await vi.waitFor(() => expect(screen.getByRole('switch', { name })).toHaveAccessibleDescription(on))
    expect(screen.getByRole('switch', { name })).toBeChecked()
    expect(sent(fetch, 'PATCH', `/api/me/contacts/${id}`)).toEqual([{ visibility }])
  })

  it('stays on the step while a switch saves, so a failure is never missed', async () => {
    let finish: (r: Response) => void = () => {}
    const { fetch } = renderPage()
    const yours = fetch.getMockImplementation()!
    fetch.mockImplementation((url: string, init?: RequestInit) =>
      init?.method === 'PATCH' ? new Promise<Response>((resolve) => (finish = resolve)) : yours(url, init),
    )
    await screen.findByLabelText('A line or two about you')
    click('Continue')
    const toggle = await screen.findByRole('switch', { name: 'Phone' })
    fireEvent.click(toggle)
    await vi.waitFor(() => expect(toggle).toHaveAttribute('aria-disabled', 'true'))

    for (const name of ['Continue', 'Back', 'Skip for now']) {
      click(name)
      expect(screen.getByText('Step 2 of 3')).toBeInTheDocument()
    }
    await act(async () => finish(new Response(JSON.stringify({ error: { code: 'internal', message: 'x' } }), { status: 500 })))

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong')
    click('Continue')
    expect(screen.getByText('Step 3 of 3')).toBeInTheDocument()
  })

  it('lists your rows in the usual order', async () => {
    renderPage()
    await screen.findByLabelText('A line or two about you')
    click('Continue')

    await screen.findByRole('switch', { name: 'Phone' })
    expect(screen.getAllByRole('switch').map((s) => s.getAttribute('aria-labelledby') && document.getElementById(s.getAttribute('aria-labelledby')!)?.textContent)).toEqual([
      'Phone',
      'Email',
      'Instagram',
    ])
  })

  it('says where to add details when there are none', async () => {
    renderPage({ 'GET /api/me/contacts': { status: 200, body: { person_id: 'darline', contacts: [] } } })
    await screen.findByLabelText('A line or two about you')
    click('Continue')

    expect(await screen.findByText('Nothing here yet. You can add a phone, email or address on My profile.')).toBeInTheDocument()
  })

  it('shows your grandparents, parents and living cousins from the tree', async () => {
    renderPage()
    await screen.findByLabelText('A line or two about you')
    click('Continue')
    click('Continue')

    expect(await screen.findByText('Lucien Pluviose')).toBeInTheDocument()
    expect(screen.getByText('Rose Pluviose')).toBeInTheDocument()
    expect(screen.getByText('Frantz Pluviose')).toBeInTheDocument()
    expect(screen.getByText('Mirlande Pluviose')).toBeInTheDocument()
    expect(screen.queryByText('Foster Parent')).not.toBeInTheDocument()
    expect(screen.getByText('Darline · You')).toBeInTheDocument()
    expect(
      screen.getByText('Your cousins Maya and Fritz are here too. Something look wrong? Let an admin know and they’ll fix it.'),
    ).toBeInTheDocument()
  })

  it('leaves out the Monestime side’s grandparents and cousins, but not a parent from it', async () => {
    renderPage({
      'GET /api/tree': {
        status: 200,
        body: {
          persons: [
            ...tree.persons.filter((p) => p.id !== 'mirlande'),
            person('mirlande', 'Mirlande', 'Pluviose', 'second'),
            person('odette', 'Odette', 'Monestime', 'second'),
            person('serge', 'Serge', 'Monestime', 'second'),
            person('kerline', 'Kerline', 'Monestime', 'second'),
          ],
          relationships: [...tree.relationships, parent('odette', 'mirlande'), parent('odette', 'serge'), parent('serge', 'kerline')],
        },
      },
    })
    await screen.findByLabelText('A line or two about you')
    click('Continue')
    click('Continue')

    expect(await screen.findByText('Lucien Pluviose')).toBeInTheDocument()
    expect(screen.getByText('Mirlande Pluviose')).toBeInTheDocument()
    expect(screen.queryByText('Odette Monestime')).not.toBeInTheDocument()
    expect(
      screen.getByText('Your cousins Maya and Fritz are here too. Something look wrong? Let an admin know and they’ll fix it.'),
    ).toBeInTheDocument()
  })

  it('counts a cousin your parents also fostered, and names three before “and N more”', async () => {
    const extra = ['sabine', 'lise', 'jean'].map((id) => person(id, id[0]!.toUpperCase() + id.slice(1), 'Pluviose'))
    renderPage({
      'GET /api/tree': {
        status: 200,
        body: {
          persons: [...tree.persons, ...extra],
          relationships: [...tree.relationships, ...extra.map((p) => parent('paul', p.id)), parent('frantz', 'maya', 'foster')],
        },
      },
    })
    await screen.findByLabelText('A line or two about you')
    click('Continue')
    click('Continue')

    expect(
      await screen.findByText('Your cousins Maya, Fritz, Sabine and 2 more are here too. Something look wrong? Let an admin know and they’ll fix it.'),
    ).toBeInTheDocument()
  })

  it('says so when your parents aren’t in the tree yet', async () => {
    renderPage({ 'GET /api/tree': { status: 200, body: { persons: [person('darline', 'Darline', 'Pluviose')], relationships: [] } } })
    await screen.findByLabelText('A line or two about you')
    click('Continue')
    click('Continue')

    expect(
      await screen.findByText('Your parents aren’t in the tree yet. Something look wrong? Let an admin know and they’ll fix it.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Grandparents')).not.toBeInTheDocument()
  })

  it('ends on the home page', async () => {
    renderPage()
    await screen.findByLabelText('A line or two about you')
    click('Continue')
    click('Continue')

    fireEvent.click(screen.getByRole('link', { name: 'Go to my home page' }))

    expect(screen.getByText('Home page')).toBeInTheDocument()
  })

  it.each([
    { name: 'goes to sign in when signed out', me: { status: 401, body: { error: { code: 'unauthorized', message: 'sign in first' } } }, lands: 'Sign in page' },
    { name: 'goes home for a login linked to nobody', me: { status: 200, body: { ...darlineLogin, person_id: null } }, lands: 'Home page' },
  ])('$name', async ({ me, lands }) => {
    renderPage({ 'GET /api/auth/me': me })

    expect(await screen.findByText(lands)).toBeInTheDocument()
  })
})
