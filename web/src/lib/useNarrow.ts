import { useSyncExternalStore } from 'react'

// Phone width, where pages lay out in one column (the same break as the CSS's 600px).
const narrowQuery = '(max-width: 600px)'

// One list per matchMedia, rather than a new one on every render.
let cached: { from: typeof window.matchMedia; list: MediaQueryList } | null = null
const list = () => {
  if (typeof window.matchMedia !== 'function') {
    return null
  }
  if (cached?.from !== window.matchMedia) {
    cached = { from: window.matchMedia, list: window.matchMedia(narrowQuery) }
  }
  return cached.list
}

const subscribe = (listener: () => void) => {
  const m = list()
  m?.addEventListener('change', listener)
  return () => m?.removeEventListener('change', listener)
}

export function useNarrow(): boolean {
  return useSyncExternalStore(subscribe, () => list()?.matches ?? false, () => false)
}
