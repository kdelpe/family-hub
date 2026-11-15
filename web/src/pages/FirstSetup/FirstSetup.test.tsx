import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { describe, expect, it } from 'vitest'

import { FirstSetup } from './FirstSetup'

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: '/setup', element: <FirstSetup /> },
      { path: '/', element: <p>Home page</p> },
    ],
    { initialEntries: ['/setup'] },
  )
  render(<RouterProvider router={router} />)
}

const click = (name: string) => fireEvent.click(screen.getByRole('button', { name }))

describe('FirstSetup', () => {
  it('walks through the three steps and back', () => {
    renderPage()
    expect(screen.getByText('Step 1 of 3')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Back' })).not.toBeInTheDocument()

    click('Continue')
    expect(screen.getByRole('heading', { name: 'How can family reach you?' })).toHaveFocus()
    expect(screen.getByText('Step 2 of 3')).toBeInTheDocument()

    click('Skip for now')
    expect(screen.getByRole('heading', { name: 'Here’s your place in the family' })).toHaveFocus()
    expect(screen.queryByRole('button', { name: 'Skip for now' })).not.toBeInTheDocument()

    click('Back')
    expect(screen.getByRole('heading', { name: 'How can family reach you?' })).toHaveFocus()
  })

  it.each([
    { name: 'Mobile', on: 'Family can ask' },
    { name: 'Email', on: 'Visible to family' },
    { name: 'Home address', on: 'Family can ask' },
    { name: 'WhatsApp', on: 'Family can ask' },
    { name: 'Instagram', on: 'Visible to family' },
  ])('starts $name as Only me and turns it on as "$on"', ({ name, on }) => {
    renderPage()
    click('Continue')
    const contact = screen.getByRole('switch', { name })
    expect(contact).not.toBeChecked()
    expect(contact).toHaveAccessibleDescription('Only me')

    fireEvent.click(contact)

    expect(contact).toBeChecked()
    expect(contact).toHaveAccessibleDescription(on)
  })

  it('ends on the home page', () => {
    renderPage()
    click('Continue')
    click('Continue')

    fireEvent.click(screen.getByRole('link', { name: 'Go to my home page' }))

    expect(screen.getByText('Home page')).toBeInTheDocument()
  })
})
