import { useSyncExternalStore } from 'react'

const foldedKey = 'fh-menu-folded'
const listeners = new Set<() => void>()

// Local storage can throw (private windows, blocked storage); the choice then lasts until the page reloads.
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

// Also hears a fold in another tab.
const subscribe = (listener: () => void) => {
  const onStorage = (event: StorageEvent) => {
    // A null key: the whole storage was cleared.
    if (event.key === foldedKey || event.key === null) {
      folded = null
      listener()
    }
  }
  listeners.add(listener)
  window.addEventListener('storage', onStorage)
  return () => {
    listeners.delete(listener)
    window.removeEventListener('storage', onStorage)
  }
}

const setFolded = (next: boolean) => {
  folded = next
  try {
    localStorage.setItem(foldedKey, next ? '1' : '0')
  } catch {
    // Kept in memory only.
  }
  listeners.forEach((l) => l())
}

// The menu folded to its rail (design-notes §2.1), kept for the next visit.
export function useMenuFolded(): [boolean, (folded: boolean) => void] {
  return [useSyncExternalStore(subscribe, readFolded, () => false), setFolded]
}

// Tests only: forget the remembered fold, as the module outlives each test.
export function resetMenuFolded() {
  folded = null
}
