import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { expect, it } from 'vitest'

import { routes } from './routes'

it.each([
  { path: '/auth/login', heading: 'Lakay se lakay' },
  { path: '/auth/request-access', heading: 'Request access' },
  { path: '/auth/forgot', heading: 'Forgot your password?' },
  { path: '/auth/set-password', heading: 'This link doesn’t work anymore' },
  { path: '/setup', heading: 'Let the family see who you are' },
  { path: '/people', heading: 'Pluviose-Monestime Family Hub' },
])('$path shows the page with heading "$heading"', async ({ path, heading }) => {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={createMemoryRouter(routes, { initialEntries: [path] })} />
    </QueryClientProvider>,
  )

  expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
})
