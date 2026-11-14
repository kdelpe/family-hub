import { render, screen } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { expect, it } from 'vitest'

import { branches, leaves, roots } from '../SignInTree/signInTreeData'
import { SignInLayout } from './SignInLayout'

it('puts the brand panel with the whole tree beside the page', () => {
  const router = createMemoryRouter([{ element: <SignInLayout />, children: [{ path: '/', element: <main>Page</main> }] }])
  const { container } = render(<RouterProvider router={router} />)

  const brand = screen.getByRole('region', { name: 'About this site' })
  expect(brand).toHaveTextContent('A private home for our family’s stories,photos and history.')
  expect(container.querySelectorAll(`circle[r]`)).toHaveLength(4 + 2 * leaves.length)
  expect(container.querySelectorAll('path[stroke-width]')).toHaveLength(1 + roots.length + branches.length)
  expect(screen.getByRole('main')).toHaveTextContent('Page')
})
