import type { Tree } from '../api/tree'

const childWord = { male: 'son', female: 'daughter' } as Record<string, string>

// "In the tree as Fritz and Fabienne’s son": where someone sits, by their parents' first names, father first
// as the mockup writes it.
export function placeInTree(tree: Tree, personId: string): string {
  const person = tree.persons.find((p) => p.id === personId)
  const parents = tree.relationships
    .flatMap((r) => (r.type === 'parent_child' && r.child_id === personId ? [r.parent_id] : []))
    .flatMap((id) => tree.persons.filter((p) => p.id === id))
    .sort((a, b) => Number(b.gender === 'male') - Number(a.gender === 'male') || a.first_name.localeCompare(b.first_name))
    .map((p) => p.first_name)
  if (!person || parents.length === 0) {
    return 'In the tree'
  }
  return `In the tree as ${parents.join(' and ')}’s ${childWord[person.gender ?? ''] ?? 'child'}`
}
