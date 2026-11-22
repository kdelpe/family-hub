import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { MotionGlobalConfig } from 'motion/react'
import { afterEach } from 'vitest'

// Animations finish at once, so a closed dialog is gone on the next frame.
MotionGlobalConfig.skipAnimations = true

afterEach(cleanup)
