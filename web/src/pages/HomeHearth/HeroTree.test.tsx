import { render } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// HomeHearth keeps "already grown this visit" at module level, so each test imports it afresh.
const fresh = async () => {
  vi.resetModules()
  return import('./HomeHearth')
}
const leafStyle = (container: HTMLElement) => container.querySelector('circle')!.getAttribute('style') ?? ''

describe('HeroTree', () => {
  beforeEach(() => localStorage.clear())
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
})
