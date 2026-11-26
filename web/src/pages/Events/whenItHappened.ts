import type { FamilyEvent } from '../../api/events'
import { bornOn } from '../../lib/dates'

// "November 13, 1984", "July 2025", "1984", or "Around 1962", as the design writes them.
export function whenItHappened(event: FamilyEvent) {
  if (event.date_precision === 'circa') {
    return `Around ${event.date}`
  }
  return bornOn(event.date, event.date_precision)
}
