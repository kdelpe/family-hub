import { useSyncExternalStore } from 'react'

const pausedKey = 'fh-motion-paused'
const listeners = new Set<() => void>()

// Session storage can throw (private windows, blocked storage); the choice then lasts until the page reloads.
let paused: boolean | null = null
const readPaused = () => {
  if (paused === null) {
    try {
      paused = sessionStorage.getItem(pausedKey) === '1'
    } catch {
      paused = false
    }
  }
  return paused
}

const subscribePaused = (listener: () => void) => {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

const setPaused = (next: boolean) => {
  paused = next
  try {
    sessionStorage.setItem(pausedKey, next ? '1' : '0')
  } catch {
    // Kept in memory only.
  }
  listeners.forEach((l) => l())
}

// One Pause motion choice for the whole site, kept for the session (design-notes §6.4).
export function useMotionPaused(): [boolean, (paused: boolean) => void] {
  return [useSyncExternalStore(subscribePaused, readPaused, () => false), setPaused]
}

const foldedKey = 'fh-menu-folded'
const foldedListeners = new Set<() => void>()
let folded: boolean | null = null
const readFolded = () => {
  if (folded === null) {
    try {
      folded = localStorage.getItem(foldedKey) === '1'
    } catch {
      folded = false
    }
  }
  return folded
}
const subscribeFolded = (listener: () => void) => {
  foldedListeners.add(listener)
  return () => foldedListeners.delete(listener)
}
const setFolded = (next: boolean) => {
  folded = next
  try {
    localStorage.setItem(foldedKey, next ? '1' : '0')
  } catch {
    // Kept in memory only.
  }
  foldedListeners.forEach((l) => l())
}

// The menu folded to its rail (design-notes §2.1), kept for the next visit.
export function useMenuFolded(): [boolean, (folded: boolean) => void] {
  return [useSyncExternalStore(subscribeFolded, readFolded, () => false), setFolded]
}

// Tests only: forget the remembered fold, as the module outlives each test.
export function resetMenuFolded() {
  folded = null
}

const reducedQuery = '(prefers-reduced-motion: reduce)'
// One media query list, made again only if matchMedia itself changes (as tests stub it).
let reduced: { from: typeof window.matchMedia; list: MediaQueryList } | null = null
const media = () => {
  if (typeof window.matchMedia !== 'function') {
    return null
  }
  if (reduced?.from !== window.matchMedia) {
    reduced = { from: window.matchMedia, list: window.matchMedia(reducedQuery) }
  }
  return reduced.list
}

const subscribeReduced = (listener: () => void) => {
  const m = media()
  m?.addEventListener('change', listener)
  return () => m?.removeEventListener('change', listener)
}
const readReduced = () => media()?.matches ?? false

export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribeReduced, readReduced, () => false)
}
