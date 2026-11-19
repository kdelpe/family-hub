export type DayPart = 'morning' | 'afternoon' | 'evening' | 'night'

export function dayPart(date: Date): DayPart {
  const hour = date.getHours()
  if (hour >= 5 && hour < 12) {
    return 'morning'
  }
  if (hour >= 12 && hour < 17) {
    return 'afternoon'
  }
  return hour >= 17 && hour < 22 ? 'evening' : 'night'
}

// "Friday evening", as the Home hero's small caps label shows it (design-notes §4.1).
export function dayPartLabel(date: Date): string {
  return `${date.toLocaleDateString('en-US', { weekday: 'long' })} ${dayPart(date)}`
}

// "Good night" is a goodbye, so the night greets as the evening does.
export function greeting(date: Date): string {
  const part = dayPart(date)
  return `Good ${part === 'night' ? 'evening' : part}`
}
