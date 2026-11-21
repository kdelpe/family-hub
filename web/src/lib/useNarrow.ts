import { useSyncExternalStore } from 'react'

// Phone width, where pages lay out in one column (the same break as the CSS's 600px).
const narrowQuery = '(max-width: 600px)'

const list = () => (typeof window.matchMedia === 'function' ? window.matchMedia(narrowQuery) : null)

const subscribe = (listener: () => void) => {
  const m = list()
  m?.addEventListener('change', listener)
  return () => m?.removeEventListener('change', listener)
}

export function useNarrow(): boolean {
  return useSyncExternalStore(subscribe, () => list()?.matches ?? false, () => false)
}
