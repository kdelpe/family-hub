import { type ContactMethod, platformLabel } from '../api/contacts'

export type ContactType = {
  id: string
  name: string
  kind: ContactMethod['kind']
  platform: string | null
  hint: string
  // Kind "other" rows come only from an admin; members add an "Other account" instead.
  addable?: false
}

const social = (id: string, name = platformLabel(id)): ContactType => ({
  id,
  name,
  kind: 'social',
  platform: id,
  hint: '@handle or a link to your profile',
})

// Every kind of row a member can have, in the mockup's order.
export const contactTypes: ContactType[] = [
  { id: 'phone', name: 'Phone', kind: 'phone', platform: null, hint: '555-010-0000' },
  { id: 'email', name: 'Email', kind: 'email', platform: null, hint: 'name@example.com' },
  { id: 'address', name: 'Address', kind: 'address', platform: null, hint: '' },
  { ...social('whatsapp'), hint: '555-010-0000' },
  social('instagram'),
  social('facebook'),
  social('tiktok'),
  social('linkedin'),
  social('x'),
  social('youtube'),
  { ...social('other', 'Other account'), hint: 'Your handle, or a link' },
  { id: 'other-kind', name: 'Other', kind: 'other', platform: null, hint: '', addable: false },
]

export const typeOf = (c: Pick<ContactMethod, 'kind' | 'platform'>) =>
  contactTypes.find((t) => t.kind === c.kind && t.platform === c.platform)

// The mockup's order, whatever order they were added in.
const rank = (c: ContactMethod) => {
  const type = typeOf(c)
  return type ? contactTypes.indexOf(type) : contactTypes.length
}
export const byType = (a: ContactMethod, b: ContactMethod) => rank(a) - rank(b)

// A row's name when it has no label of its own: "Phone", "Instagram".
export const typeName = (c: ContactMethod) =>
  c.kind === 'social' ? platformLabel(c.platform) : (typeOf(c)?.name ?? 'Other')
