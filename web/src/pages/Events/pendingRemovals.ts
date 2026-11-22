import { useSyncExternalStore } from 'react'

// Outside the query cache, so a refetch during the Undo countdown can't bring an event back.
let pending: ReadonlySet<string> = new Set()
const listeners = new Set<() => void>()

const update = (next: Set<string>) => {
  pending = next
  listeners.forEach((l) => l())
}

export const hideEvent = (id: string) => update(new Set(pending).add(id))

export const unhideEvent = (id: string) => {
  const next = new Set(pending)
  next.delete(id)
  update(next)
}

const subscribe = (listener: () => void) => {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export const usePendingRemovals = () => useSyncExternalStore(subscribe, () => pending)
