import { fireEvent, render, screen } from '@testing-library/react'
import { LazyMotion, domMax, MotionGlobalConfig } from 'motion/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ConfirmDialog } from './ConfirmDialog'

describe('ConfirmDialog', () => {
  afterEach(() => {
    MotionGlobalConfig.skipAnimations = true
  })

  it('ignores a choice made on the copy fading out after cancelling', () => {
    MotionGlobalConfig.skipAnimations = false
    const onYes = vi.fn()
    const onCancel = vi.fn()
    const dialog = (open: boolean) => (
      <LazyMotion features={domMax}>
        <ConfirmDialog open={open} question="Share it?" yes="Yes, share" no="Not now" busy={false} onYes={onYes} onCancel={onCancel} />
      </LazyMotion>
    )
    const { rerender } = render(dialog(true))
    const yes = screen.getByRole('button', { name: 'Yes, share' })

    rerender(dialog(false))
    fireEvent.click(yes)

    expect(yes).toBeInTheDocument()
    expect(onYes).not.toHaveBeenCalled()
  })
})
