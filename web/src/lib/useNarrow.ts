import { useSyncExternalStore } from 'react'

// One list per matchMedia, rather than a new one on every render.
function mediaHook(query: string) {
  let cached: { from: typeof window.matchMedia; list: MediaQueryList } | null = null
  const list = () => {
    if (typeof window.matchMedia !== 'function') {
      return null
    }
    if (cached?.from !== window.matchMedia) {
      cached = { from: window.matchMedia, list: window.matchMedia(query) }
    }
    return cached.list
  }
  const subscribe = (listener: () => void) => {
    const m = list()
    m?.addEventListener('change', listener)
    return () => m?.removeEventListener('change', listener)
  }
  return () => useSyncExternalStore(subscribe, () => list()?.matches ?? false, () => false)
}

// Phone width, where pages lay out in one column (the same break as the CSS's 600px).
export const useNarrow: () => boolean = mediaHook('(max-width: 600px)')

// Up to 800px the menu sits above the page, not beside it, as the pages' own 800px rules expect: no rail.
export const useMenuAbove: () => boolean = mediaHook('(max-width: 800px)')
