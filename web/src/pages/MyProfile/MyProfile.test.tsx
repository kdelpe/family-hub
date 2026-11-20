import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { authKeys, type User } from '../../api/auth'
import type { ContactMethod } from '../../api/contacts'
import { type Person, personKeys } from '../../api/persons'
import { MyProfile } from './MyProfile'

const maya: Person = {
  id: 'maya',
  first_name: 'Maya',
  last_name: 'Pluviose',
  maiden_name: null,
  birth_place: null,
  birth_date: '1978-06-06',
  birth_date_precision: 'day',
  death_date: null,
  death_date_precision: null,
  is_deceased: false,
  bio: 'Keeper of the family recipes.',
}

const method = (over: Partial<ContactMethod> & Pick<ContactMethod, 'id' | 'kind' | 'value'>): ContactMethod => ({
  platform: null,
  label: null,
  visibility: 'hidden',
  href: null,
  shared_with_you: false,
  ...over,
})

const contacts: ContactMethod[] = [
  method({ id: 'c1', kind: 'social', platform: 'instagram', value: '@maya.p' }),
  method({ id: 'c2', kind: 'email', value: 'maya@example.com', visibility: 'family' }),
  method({ id: 'c3', kind: 'phone', label: 'Mobile', value: '(555) 010-7781' }),
]

type Answer = { status: number; body?: unknown }

function renderPage(answers: Record<string, Answer> = {}, user: Partial<User> = {}, person: Person | null = maya) {
  const fetch = vi.fn((url: string, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${url}`
    // Read at call time, so a test can change an answer midway.
    const answer = answers[key] ?? (key === 'GET /api/me/contacts' ? { status: 200, body: { contacts } } : undefined)
    return answer
      ? Promise.resolve(new Response(answer.body === undefined ? null : JSON.stringify(answer.body), { status: answer.status }))
      : Promise.reject(new TypeError('Failed to fetch'))
  })
  vi.stubGlobal('fetch', fetch)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(authKeys.me, { id: 'u', email: 'maya@example.com', role: 'member', person_id: 'maya', ...user })
  if (person) {
    client.setQueryData(personKeys.one('maya'), person)
  }
  const router = createMemoryRouter(
    [
      { path: '/profile', element: <MyProfile /> },
      { path: '/auth/login', element: <p>Sign in page</p> },
    ],
    { initialEntries: ['/profile'] },
  )
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { fetch, client }
}

const sent = (fetch: ReturnType<typeof vi.fn>, method: string, url: string) =>
  fetch.mock.calls.find(([u, init]) => u === url && (init as RequestInit | undefined)?.method === method)?.[1] as
    | RequestInit
    | undefined

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('MyProfile', () => {
  it('shows who you are, your story and your contacts in the mockup’s order', async () => {
    renderPage()

    expect(screen.getByText('Maya Pluviose')).toBeInTheDocument()
    expect(screen.getByText('Born June 6, 1978')).toBeInTheDocument()
    expect(screen.getByLabelText('Your story')).toHaveValue('Keeper of the family recipes.')
    const switches = await screen.findAllByRole('switch')
    expect(switches.map((s) => s.textContent)).toEqual([
      'Only me',
      'Visible to family',
      'Only me',
    ])
    expect(screen.getByRole('switch', { name: 'Mobile' })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Instagram' })).toBeInTheDocument()
  })

  it('saves the story only once it changed', async () => {
    const { fetch } = renderPage({
      'PATCH /api/persons/maya': { status: 200, body: { ...maya, bio: 'Nurse in Boston.' } },
    })

    expect(screen.getByRole('button', { name: 'Save' })).toHaveAttribute('aria-disabled', 'true')
    fireEvent.change(screen.getByLabelText('Your story'), { target: { value: '  Nurse in Boston. ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Saved.')).toBeInTheDocument()
    expect(sent(fetch, 'PATCH', '/api/persons/maya')?.body).toBe(JSON.stringify({ bio: 'Nurse in Boston.' }))
    expect(screen.getByLabelText('Your story')).toHaveValue('Nurse in Boston.')
    // Still there for the next edit, but nothing to save until the text changes.
    expect(screen.getByRole('button', { name: 'Save' })).toHaveAttribute('aria-disabled', 'true')
    expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument()
  })

  it.each([
    { name: 'a phone can be asked for', row: 'Mobile', id: 'c3', want: 'on_request', status: 'Family can ask' },
    { name: 'a social account is shown', row: 'Instagram', id: 'c1', want: 'family', status: 'Visible to family' },
    { name: 'turning off hides it', row: 'Email', id: 'c2', want: 'hidden', status: 'Only me' },
  ])('switches: $name', async ({ row, id, want, status }) => {
    const before = contacts.find((c) => c.id === id)!
    const { fetch } = renderPage({
      [`PATCH /api/me/contacts/${id}`]: { status: 200, body: { ...before, visibility: want } },
    })

    fireEvent.click(await screen.findByRole('switch', { name: row }))

    await vi.waitFor(() => expect(screen.getByRole('switch', { name: row })).toHaveTextContent(status))
    expect(screen.getByRole('switch', { name: row })).toHaveAttribute('aria-checked', String(want !== 'hidden'))
    expect(sent(fetch, 'PATCH', `/api/me/contacts/${id}`)?.body).toBe(JSON.stringify({ visibility: want }))
  })

  it('adds a contact as “Only me”, and says what the server refused', async () => {
    const { fetch } = renderPage({
      'POST /api/me/contacts': {
        status: 400,
        body: { error: { code: 'validation_error', message: 'invalid contact method: the value must be an email address like name@example.com' } },
      },
    })

    fireEvent.click(await screen.findByRole('button', { name: '+ Add a phone, email, address or social account' }))
    const form = within(screen.getByRole('form', { name: 'Add a contact' }))
    fireEvent.change(form.getByLabelText('Type'), { target: { value: 'email' } })
    fireEvent.change(form.getByLabelText('Email'), { target: { value: 'maya@' } })
    fireEvent.change(form.getByLabelText(/Label/), { target: { value: ' Work ' } })
    fireEvent.click(form.getByRole('button', { name: 'Save' }))

    expect(await form.findByRole('alert')).toHaveTextContent('The value must be an email address like name@example.com.')
    expect(sent(fetch, 'POST', '/api/me/contacts')?.body).toBe(
      JSON.stringify({ kind: 'email', platform: null, visibility: 'hidden', label: 'Work', value: 'maya@' }),
    )
  })

  it('puts dashes in a phone number as it’s typed', async () => {
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: '+ Add a phone, email, address or social account' }))
    const value = within(screen.getByRole('form', { name: 'Add a contact' })).getByLabelText('Phone')
    fireEvent.change(value, { target: { value: '1111111111' } })

    expect(value).toHaveValue('111-111-1111')
  })

  it('edits and removes a contact', async () => {
    const { fetch } = renderPage({
      'PATCH /api/me/contacts/c3': { status: 200, body: { ...contacts[2], label: 'Cell' } },
      'DELETE /api/me/contacts/c3': { status: 204 },
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Edit Mobile' }))
    let form = within(screen.getByRole('form', { name: 'Edit Phone' }))
    fireEvent.change(form.getByLabelText(/Label/), { target: { value: 'Cell' } })
    fireEvent.click(form.getByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('switch', { name: 'Cell' })).toBeInTheDocument()
    expect(sent(fetch, 'PATCH', '/api/me/contacts/c3')?.body).toBe(JSON.stringify({ label: 'Cell', value: '(555) 010-7781' }))

    fireEvent.click(screen.getByRole('button', { name: 'Edit Cell' }))
    form = within(screen.getByRole('form', { name: 'Edit Phone' }))
    fireEvent.click(form.getByRole('button', { name: 'Remove' }))
    expect(fetch).not.toHaveBeenCalledWith('/api/me/contacts/c3', expect.objectContaining({ method: 'DELETE' }))
    fireEvent.click(form.getByRole('button', { name: 'Yes, remove' }))
    await vi.waitFor(() => expect(screen.queryByRole('switch', { name: 'Cell' })).not.toBeInTheDocument())
    expect(screen.getAllByRole('switch')).toHaveLength(2)
  })

  it('changes the password, and says when the current one is wrong', async () => {
    const answers: Record<string, Answer> = {
      'POST /api/auth/password/change': {
        status: 400,
        body: { error: { code: 'wrong_password', message: 'current password is incorrect' } },
      },
    }
    const { fetch } = renderPage(answers)

    fireEvent.click(screen.getByRole('button', { name: 'Change password' }))
    const savePassword = () => within(screen.getByRole('form', { name: 'Change password' })).getByRole('button', { name: 'Save' })
    fireEvent.change(screen.getByLabelText('Current password'), { target: { value: 'old one here' } })
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'short' } })
    fireEvent.click(savePassword())
    expect(screen.getByText('Use at least 8 characters.')).toBeInTheDocument()
    expect(sent(fetch, 'POST', '/api/auth/password/change')).toBeUndefined()

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'a long new phrase' } })
    fireEvent.click(savePassword())
    expect(await screen.findByText('That’s not your current password.')).toBeInTheDocument()

    answers['POST /api/auth/password/change'] = { status: 204 }
    fireEvent.click(savePassword())
    expect(await screen.findByText('Password changed. You’re signed out on your other devices.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Current password')).not.toBeInTheDocument()
  })

  it('signs out and forgets everything fetched', async () => {
    const { client } = renderPage({ 'POST /api/auth/logout': { status: 204 } })

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))

    expect(await screen.findByText('Sign in page')).toBeInTheDocument()
    expect(client.getQueryData(personKeys.one('maya'))).toBeUndefined()
  })

  it('says so when the login isn’t linked to anyone yet', () => {
    const { fetch } = renderPage({}, { person_id: null })

    expect(screen.getByText(/Your login isn’t linked to you in the tree yet/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'See me in the tree' })).not.toBeInTheDocument()
    expect(fetch).not.toHaveBeenCalledWith('/api/me/contacts', expect.anything())
  })

  it('says the profile didn’t load, not that the login isn’t linked', async () => {
    renderPage({ 'GET /api/persons/maya': { status: 503, body: { error: { code: 'busy', message: 'x' } } } }, {}, null)

    expect(await screen.findByText('Your profile didn’t load. Refresh the page to try again.')).toBeInTheDocument()
    expect(screen.queryByText(/isn’t linked/)).not.toBeInTheDocument()
  })

  it('rechecks the session when a save finds it ended', async () => {
    const { fetch } = renderPage({
      'PATCH /api/me/contacts/c3': { status: 401, body: { error: { code: 'unauthorized', message: 'x' } } },
      'GET /api/auth/me': { status: 401, body: { error: { code: 'unauthorized', message: 'x' } } },
    })

    const checks = () => fetch.mock.calls.filter(([url]) => url === '/api/auth/me').length
    const toggle = await screen.findByRole('switch', { name: 'Mobile' })
    await vi.waitFor(() => expect(checks()).toBe(1))

    fireEvent.click(toggle)

    await vi.waitFor(() => expect(checks()).toBe(2))
  })

  it('counts a failed sign-out answer as signed out, since the cookie is already gone', async () => {
    renderPage({ 'POST /api/auth/logout': { status: 500, body: { error: { code: 'internal', message: 'x' } } } })

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))

    expect(await screen.findByText('Sign in page')).toBeInTheDocument()
  })

  it('starts the value afresh when the type changes', async () => {
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: '+ Add a phone, email, address or social account' }))
    const form = within(screen.getByRole('form', { name: 'Add a contact' }))
    fireEvent.change(form.getByLabelText('Phone'), { target: { value: '1115550000' } })
    fireEvent.change(form.getByLabelText('Type'), { target: { value: 'email' } })

    expect(form.getByLabelText('Email')).toHaveValue('')
  })

  it('lets you fix your name and birthday, sending only what changed', async () => {
    const { fetch } = renderPage({
      'PATCH /api/persons/maya': { status: 200, body: { ...maya, last_name: 'Pluviose-Jean', birth_date: '1978-07-06' } },
    })

    fireEvent.click(screen.getByRole('button', { name: 'Edit your name, birthday and photo' }))
    const form = within(screen.getByRole('form', { name: 'Your details' }))
    expect(form.getByText('Photos are coming soon.')).toBeInTheDocument()
    expect(form.getByLabelText('First name')).toHaveValue('Maya')
    expect(form.getByLabelText('Birthday')).toHaveValue('1978-06-06')
    fireEvent.change(form.getByLabelText('Last name'), { target: { value: 'Pluviose-Jean ' } })
    fireEvent.change(form.getByLabelText('Birthday'), { target: { value: '1978-07-06' } })
    fireEvent.click(form.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Born July 6, 1978')).toBeInTheDocument()
    expect(screen.getByText('Maya Pluviose-Jean')).toBeInTheDocument()
    expect(sent(fetch, 'PATCH', '/api/persons/maya')?.body).toBe(
      JSON.stringify({ last_name: 'Pluviose-Jean', birth_date: '1978-07-06', birth_date_precision: 'day' }),
    )
  })

  it('opens the same form from the photo', () => {
    renderPage()

    fireEvent.click(screen.getByText('MP'))

    expect(screen.getByRole('form', { name: 'Your details' })).toBeInTheDocument()
  })

  it('needs a first name, and keeps a partial birthday unless a full one is picked', async () => {
    const yearOnly = { ...maya, birth_date: '1978', birth_date_precision: 'year' as const }
    const { fetch } = renderPage({ 'PATCH /api/persons/maya': { status: 200, body: { ...yearOnly, first_name: 'May' } } }, {}, yearOnly)

    fireEvent.click(screen.getByRole('button', { name: 'Edit your name, birthday and photo' }))
    const form = within(screen.getByRole('form', { name: 'Your details' }))
    expect(form.getByText('On file: 1978. Pick the full date to change it.')).toBeInTheDocument()
    fireEvent.change(form.getByLabelText('First name'), { target: { value: '  ' } })
    fireEvent.click(form.getByRole('button', { name: 'Save' }))
    expect(form.getByText('Enter your first name.')).toBeInTheDocument()

    fireEvent.change(form.getByLabelText('First name'), { target: { value: 'May' } })
    fireEvent.click(form.getByRole('button', { name: 'Save' }))
    expect(await screen.findByText('May Pluviose')).toBeInTheDocument()
    expect(sent(fetch, 'PATCH', '/api/persons/maya')?.body).toBe(JSON.stringify({ first_name: 'May' }))
  })

  it.each([
    { name: 'emptying the date clears it', date: '', want: { birth_date: null, birth_date_precision: null } },
    { name: 'a future date is caught first', date: '2999-01-01', want: null },
  ])('birthday: $name', async ({ date, want }) => {
    const { fetch } = renderPage({
      'PATCH /api/persons/maya': { status: 200, body: { ...maya, birth_date: null, birth_date_precision: null } },
    })

    fireEvent.click(screen.getByRole('button', { name: 'Edit your name, birthday and photo' }))
    const form = within(screen.getByRole('form', { name: 'Your details' }))
    fireEvent.change(form.getByLabelText('Birthday'), { target: { value: date } })
    fireEvent.click(form.getByRole('button', { name: 'Save' }))

    if (want) {
      expect(await screen.findByText('No birthday yet')).toBeInTheDocument()
      expect(sent(fetch, 'PATCH', '/api/persons/maya')?.body).toBe(JSON.stringify(want))
    } else {
      expect(form.getByText('Pick a date that isn’t in the future.')).toBeInTheDocument()
      expect(sent(fetch, 'PATCH', '/api/persons/maya')).toBeUndefined()
    }
  })

  it('says when there’s no birthday yet', () => {
    renderPage({}, {}, { ...maya, birth_date: null, birth_date_precision: null })

    expect(screen.getByText('No birthday yet')).toBeInTheDocument()
  })

  it('asks for the current password before using up a try', () => {
    const { fetch } = renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Change password' }))
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'a long new phrase' } })
    fireEvent.click(within(screen.getByRole('form', { name: 'Change password' })).getByRole('button', { name: 'Save' }))

    expect(screen.getByText('Enter your current password.')).toBeInTheDocument()
    expect(sent(fetch, 'POST', '/api/auth/password/change')).toBeUndefined()
  })
})
