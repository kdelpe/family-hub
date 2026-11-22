import { LazyMotion, MotionConfig } from 'motion/react'
import type { ReactNode } from 'react'

import { useMotionPaused } from '../lib/motion'

// Motion follows the device's reduced-motion setting, and the site's own Pause motion too (design-notes §6.4).
// LazyMotion with strict: components use the slim `m.*`, and the animation features load in their own file after the page.
const loadFeatures = () => import('./motionFeatures').then((mod) => mod.default)

export function MotionPreferences({ children }: { children: ReactNode }) {
  const [paused] = useMotionPaused()
  return (
    <LazyMotion features={loadFeatures} strict>
      <MotionConfig reducedMotion={paused ? 'always' : 'user'}>{children}</MotionConfig>
    </LazyMotion>
  )
}
