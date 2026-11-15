// The mockup's made-up sample, shown until Phase 3 serves the signed-in person's own profile, contacts and family.
// Nothing here is saved.

type ContactRow = {
  id: string
  label: string
  value: string
  // A phone, WhatsApp or address is shared on request; email and social accounts are simply visible (§5.2).
  sharedOnRequest: boolean
}

export const sampleSetup = {
  firstName: 'Darline',
  initials: 'DM',
  bio: 'Frantz and Mirlande’s daughter. Physical therapist in Miami. Always bringing the pikliz.',
  contacts: [
    { id: 'mobile', label: 'Mobile', value: '(555) 010-6624', sharedOnRequest: true },
    { id: 'email', label: 'Email', value: 'darline.monestime@example.com', sharedOnRequest: false },
    { id: 'address', label: 'Home address', value: '15 Palm Way, Miami, FL', sharedOnRequest: true },
    { id: 'whatsapp', label: 'WhatsApp', value: '(555) 010-6624', sharedOnRequest: true },
    { id: 'instagram', label: 'Instagram', value: '@darline.m', sharedOnRequest: false },
  ] satisfies ContactRow[],
  grandparents: ['Lucien Monestime', 'Rose Monestime'],
  parents: ['Frantz Monestime', 'Mirlande Monestime'],
  familyNote: 'Your cousins Maya, Fritz and Sabine are on the Pluviose side.',
}
