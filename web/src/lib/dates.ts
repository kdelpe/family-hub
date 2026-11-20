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
