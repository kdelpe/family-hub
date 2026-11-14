import { render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { expect, test } from 'vitest'

import { routes } from './routes'

test('any path shows the placeholder until the pages land', () => {
  render(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ['/auth/login'] })} />)

  expect(screen.getByRole('heading', { name: 'Pluviose-Monestime Family Hub' })).toBeInTheDocument()
})
