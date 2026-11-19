import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { describe, expect, it } from 'vitest'

import { authKeys } from '../../api/auth'
import { personKeys } from '../../api/persons'
import { HomeHearth } from './HomeHearth'

const fridayEvening = new Date('2026-11-20T18:30')

function renderHome(personId: string | null) {
  const client = new QueryClient()
  client.setQueryData(authKeys.me, { id: 'u1', email: 'maya.pluviose@example.com', role: 'admin', person_id: personId })
  client.setQueryData(personKeys.one('p1'), { id: 'p1', first_name: 'Maya', last_name: 'Pluviose' })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={createMemoryRouter([{ path: '/', element: <HomeHearth now={fridayEvening} /> }])} />
    </QueryClientProvider>,
  )
}

describe('HomeHearth', () => {
  it('greets the viewer by name for the time of day', () => {
    renderHome('p1')

    expect(screen.getByText('Friday evening')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'Good evening, Maya' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open the family tree' })).toHaveAttribute('href', '/tree')
    expect(screen.getByRole('link', { name: 'See new photos' })).toHaveAttribute('href', '/events')
  })

  it('greets a login linked to nobody without a name', () => {
    renderHome(null)

    expect(screen.getByRole('heading', { level: 1, name: 'Good evening' })).toBeInTheDocument()
  })
})
