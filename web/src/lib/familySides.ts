import type { FamilySide } from '../api/accessRequests'

export const familyName = 'Pluviose'

// The hub follows one family (owner's call, 2026-11-21): the data keeps a second side so it can come back, but
// nobody is offered or shown one, and the Monestime side counts as family.
export const familySides: { value: FamilySide; label: string }[] = [
  { value: 'first', label: `${familyName} family` },
  { value: 'married_in', label: 'Married into the family' },
]

// Any side but married in, the kept-but-unoffered second one included, reads as the family.
export const sideLabel = (side: FamilySide) => familySides.find((s) => s.value === (side === 'married_in' ? side : 'first'))!.label
