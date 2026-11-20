import type { Person } from '../api/persons'

const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

// "Birthday November 19" (or "Nov 19" short) from a masked date; null when there's no month to show.
export function birthday(masked: string | null, style: 'long' | 'short' = 'long'): string | null {
  const m = masked?.match(/^(?:\d{4}|-)-(\d{2})(?:-(\d{2}))?$/)
  const month = m ? months[Number(m[1]) - 1] : undefined
  if (!m || !month) {
    return null
  }
  const name = style === 'short' ? month.slice(0, 3) : month
  return m[2] ? `${name} ${Number(m[2])}` : name
}

// "June 6, 1968", "June 1968", "c. 1968", or "June 6" when the year is hidden; null when unknown.
export function bornOn(date: string | null, precision: Person['birth_date_precision']): string | null {
  const m = date?.match(/^(\d{4}|-)(?:-(\d{2})(?:-(\d{2}))?)?$/)
  if (!m) {
    return null
  }
  const y = m[1] === '-' ? null : m[1]
  const month = m[2] && precision !== 'year' && precision !== 'circa' ? months[Number(m[2]) - 1] : undefined
  if (!month) {
    return y ? (precision === 'circa' ? `c. ${y}` : y) : null
  }
  const monthDay = m[3] && precision === 'day' ? `${month} ${Number(m[3])}` : month
  if (!y) {
    return monthDay
  }
  return monthDay === month ? `${month} ${y}` : `${monthDay}, ${y}`
}

export const year = (date: string | null, precision: Person['birth_date_precision']) => {
  const y = date?.match(/^\d{4}/)?.[0]
  return y ? `${precision === 'circa' ? 'c. ' : ''}${y}` : null
}

// "c. 1935 – 1998" for someone who has passed: years only, as the design shows them.
export function lifespan(p: Person): string | null {
  const born = year(p.birth_date, p.birth_date_precision)
  const died = year(p.death_date, p.death_date_precision)
  if (!born && !died) {
    return null
  }
  return `${born ?? '?'} – ${died ?? '?'}`
}

const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

// "Today", "Tomorrow", "In 6 days".
export const inDays = (days: number) => (days === 0 ? 'Today' : days === 1 ? 'Tomorrow' : `In ${days} days`)

// "Nov 19" for a YYYY-MM-DD date, as on Coming up's date tiles.
export function monthAndDay(date: string): { month: string; day: number } {
  const [, m = '1', d = '1'] = date.split('-')
  return { month: months[Number(m) - 1]!.slice(0, 3), day: Number(d) }
}

// "today", "yesterday", "Tuesday" within the week, then "Nov 3": when someone was added, on the viewer's clock.
export function addedOn(addedAt: string, now: Date): string {
  const added = new Date(addedAt)
  const midnight = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const days = Math.round((midnight(now) - midnight(added)) / 86_400_000)
  if (days <= 0) {
    return 'today'
  }
  if (days === 1) {
    return 'yesterday'
  }
  if (days < 7) {
    return weekdays[added.getDay()]!
  }
  return `${months[added.getMonth()]!.slice(0, 3)} ${added.getDate()}`
}
