import type { FamilySide } from '../api/accessRequests'

// The two family names will come from FAMILY_SIDES once the server shares its settings; the sample family's for now.
export const familyLines = { first: 'Pluviose', second: 'Monestime' } as const

export const familySides: { value: FamilySide; label: string }[] = [
  { value: 'first', label: `${familyLines.first} side` },
  { value: 'second', label: `${familyLines.second} side` },
  { value: 'married_in', label: 'Married into the family' },
]

export const sideLabel = (side: FamilySide) => familySides.find((s) => s.value === side)?.label ?? side

// A People card's side tag: someone can also be on both sides, and "Married in" is the card's shorter wording.
export const sideTags: Record<FamilySide | 'both', string> = {
  first: sideLabel('first'),
  second: sideLabel('second'),
  both: 'Both sides',
  married_in: 'Married in',
}
