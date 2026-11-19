export const fullName = (p: { first_name: string; last_name: string | null }) =>
  p.last_name ? `${p.first_name} ${p.last_name}` : p.first_name

export const initials = (p: { first_name: string; last_name: string | null }) =>
  (p.first_name.charAt(0) + (p.last_name?.charAt(0) ?? '')).toUpperCase()

// Lowercase, accents and punctuation folded away, so "nadege" finds Nadège and "jean louis" finds Jean-Louis.
export const foldName = (s: string) =>
  s
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, ' ')
    .trim()

// Every word typed starts a word of the name, in any order.
export function nameMatches(name: string, query: string): boolean {
  const words = foldName(name).split(' ')
  return foldName(query)
    .split(' ')
    .filter(Boolean)
    .every((typed) => words.some((word) => word.startsWith(typed)))
}
