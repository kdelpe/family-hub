import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { afterEach, expect, it, vi } from 'vitest'

import { routes } from './routes'
import { adminPages, menuPages } from './signedInPages'

const maya = { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: 'p1' }

// The API as a signed-out visitor meets it, until they sign in.
function stubApi() {
  let signedIn = false
  const json = (status: number, body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status }))
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      switch (url) {
        case '/api/auth/login':
          signedIn = true
          return json(200, maya)
        case '/api/auth/me':
          return signedIn ? json(200, maya) : json(401, { error: { code: 'unauthorized', message: 'sign in first' } })
        case '/api/persons/p1':
          return json(200, { id: 'p1', first_name: 'Maya', last_name: 'Pluviose' })
        default:
          return json(404, { error: { code: 'not_found', message: 'not found' } })
      }
    }),
  )
  return {
    signIn: () => {
      signedIn = true
    },
  }
}

function renderAt(path: string) {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <RouterProvider router={createMemoryRouter(routes, { initialEntries: [path] })} />
    </QueryClientProvider>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

it.each([
  { path: '/auth/login', heading: 'Lakay se lakay' },
  { path: '/auth/request-access', heading: 'Request access' },
  { path: '/auth/forgot', heading: 'Forgot your password?' },
  { path: '/auth/set-password', heading: 'This link doesn’t work anymore' },
  { path: '/setup', heading: 'Let the family see who you are' },
  { path: '/no-such-page', heading: 'Pluviose-Monestime Family Hub' },
])('$path shows the page with heading "$heading"', async ({ path, heading }) => {
  renderAt(path)

  expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
})

it.each([...menuPages, ...adminPages])('$path ($label) opens inside the menu', async ({ path, label }) => {
  stubApi().signIn()
  renderAt(path)

  const menu = await screen.findByRole('navigation', { name: 'Main' })
  expect(within(menu).getByRole('link', { name: label })).toHaveAttribute('aria-current', 'page')
})

it('signing in from a page you were sent away from brings you back to it', async () => {
  stubApi()
  renderAt('/people')

  fireEvent.change(await screen.findByLabelText('Email'), { target: { value: maya.email } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'sample-family-dev' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

  const menu = await screen.findByRole('navigation', { name: 'Main' })
  expect(within(menu).getByRole('link', { name: 'People' })).toHaveAttribute('aria-current', 'page')
})
