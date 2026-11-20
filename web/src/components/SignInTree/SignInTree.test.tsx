import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'

import { useMotionPaused } from '../../lib/motion'
import { SignInTree } from './SignInTree'

function PauseSwitch() {
  const [paused, setPaused] = useMotionPaused()
  return <button onClick={() => setPaused(!paused)}>{paused ? 'Play motion' : 'Pause motion'}</button>
}

it('stops its looping motion while the site is paused', () => {
  const { container } = render(
    <>
      <PauseSwitch />
      <SignInTree />
    </>,
  )
  const tree = container.querySelector('svg')!
  const before = tree.getAttribute('class')

  fireEvent.click(screen.getByRole('button', { name: 'Pause motion' }))
  expect(tree.getAttribute('class')).not.toBe(before)

  fireEvent.click(screen.getByRole('button', { name: 'Play motion' }))
  expect(tree.getAttribute('class')).toBe(before)
})
