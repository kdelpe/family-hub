import type { FamilySide } from '../api/accessRequests'

// The labels will come from FAMILY_SIDES once the server shares its settings; the sample family's for now.
export const familySides: { value: FamilySide; label: string }[] = [
  { value: 'first', label: 'Pluviose side' },
  { value: 'second', label: 'Monestime side' },
  { value: 'married_in', label: 'Married into the family' },
]

export const sideLabel = (side: FamilySide) => familySides.find((s) => s.value === side)?.label ?? side
