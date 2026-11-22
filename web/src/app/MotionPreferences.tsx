import { LazyMotion, MotionConfig } from 'motion/react'
import type { ReactNode } from 'react'

import { useMotionPaused } from '../lib/motion'

const loadFeatures = () => import('./motionFeatures').then((mod) => mod.default)

// Strict: components must use `m.*`, not `motion.*`.
export function MotionPreferences({ children }: { children: ReactNode }) {
  const [paused] = useMotionPaused()
  return (
    <LazyMotion features={loadFeatures} strict>
      <MotionConfig reducedMotion={paused ? 'always' : 'user'}>{children}</MotionConfig>
    </LazyMotion>
  )
}
