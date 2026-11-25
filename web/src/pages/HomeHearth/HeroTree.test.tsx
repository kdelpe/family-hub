import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// HomeHearth keeps "already grown this visit" at module level, so each test imports it afresh.
const fresh = async () => {
  vi.resetModules()
  return import('./HomeHearth')
}
const leafStyle = (container: HTMLElement) => container.querySelector('circle')!.getAttribute('style') ?? ''

describe('HeroTree', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })
  afterEach(() => vi.unstubAllGlobals())

  it('grows on the first arrival of a visit, and shows grown when Home is opened again', async () => {
    const { HeroTree } = await fresh()

    const first = render(<HeroTree />)
    expect(leafStyle(first.container)).toContain('scale(0)')
    first.unmount()

    const again = render(<HeroTree />)
    expect(leafStyle(again.container)).not.toContain('scale(0)')
  })

  it('shows the tree grown with reduced motion', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('reduce'),
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }))
    const { HeroTree } = await fresh()

    const { container } = render(<HeroTree />)

    expect(leafStyle(container)).not.toContain('scale(0)')
  })

  it.each([
    { name: 'sways while motion runs', still: false, sways: true },
    { name: 'holds still when paused or reduced', still: true, sways: false },
  ])('a leaf $name', async ({ still, sways }) => {
    const { leafMotion } = await fresh()

    const { animate, transition } = leafMotion(2, false, still)

    expect(Array.isArray(animate.y)).toBe(sways)
    expect('repeat' in transition.y).toBe(sways)
  })

  // With Motion running for real, a leaf's sway shows as a moving translateY; still, it settles at 0.
  describe('with the site’s Pause motion', () => {
    async function mountSwaying(startPaused: boolean) {
      const { HeroTree } = await fresh()
      const { useMotionPaused } = await import('../../lib/motion')
      const { domMax, LazyMotion, MotionGlobalConfig } = await import('motion/react')
      MotionGlobalConfig.skipAnimations = false
      function PauseSwitch() {
        const [paused, setPaused] = useMotionPaused()
        return <button onClick={() => setPaused(!paused)}>{paused ? 'Play motion' : 'Pause motion'}</button>
      }
      // Grown once already, so this mount sways straight away instead of budding first.
      render(<HeroTree />).unmount()
      const view = render(
        <LazyMotion features={domMax}>
          <PauseSwitch />
          <HeroTree />
        </LazyMotion>,
      )
      if (startPaused) {
        fireEvent.click(screen.getByRole('button', { name: 'Pause motion' }))
      }
      const offset = () => {
        const moved = view.container.querySelector('circle')!.style.transform.match(/translateY\((-?[\d.]+)px\)/)
        return moved ? Math.abs(Number(moved[1])) : 0
      }
      return { offset, restore: () => (MotionGlobalConfig.skipAnimations = true) }
    }
    const wait = (ms: number) => act(() => new Promise((r) => setTimeout(r, ms)))

    it('stops the sway when paused and starts it again on play', async () => {
      const { offset, restore } = await mountSwaying(false)
      try {
        await wait(800)
        expect(offset()).toBeGreaterThan(0.05)

        fireEvent.click(screen.getByRole('button', { name: 'Pause motion' }))
        await wait(800)
        expect(offset()).toBeLessThan(0.01)

        fireEvent.click(screen.getByRole('button', { name: 'Play motion' }))
        await wait(800)
        expect(offset()).toBeGreaterThan(0.05)
      } finally {
        restore()
      }
    })

    it('doesn’t sway in a session already paused', async () => {
      const { offset, restore } = await mountSwaying(true)
      try {
        await wait(800)
        expect(offset()).toBeLessThan(0.01)
      } finally {
        restore()
      }
    })
  })
})
