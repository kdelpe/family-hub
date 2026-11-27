import type { Story } from '../../api/stories'
import { bornOn } from '../../lib/dates'

// The same person or story always gets the same colour.
export const pick = <T,>(list: T[], id: string) => list[[...id].reduce((n, c) => (n * 31 + c.charCodeAt(0)) >>> 0, 0) % list.length]!
export const stories = (n: number) => `${n} ${n === 1 ? 'story' : 'stories'}`
export const recorded = (s: Story) => bornOn(s.recorded_on, s.recorded_on_precision)

// "4:05", or "1:02:09" past an hour.
export function timestamp(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = String(seconds % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`
}

// "Your father", from the tree's "Father"; "You" stays as it is.
export const yourRelation = (relation: string) => (relation === 'You' ? relation : `Your ${relation.toLowerCase()}`)
