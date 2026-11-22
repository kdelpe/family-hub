// The signed-in pages, in menu order (design-notes §2.1). The menu and the routes both read this list, so a
// menu link always lands inside the shell.
export const menuPages = [
  { path: '/', label: 'Home' },
  { path: '/tree', label: 'Family tree' },
  { path: '/members', label: 'Members' },
  { path: '/events', label: 'Events' },
  { path: '/stories', label: 'Stories' },
]

export const adminPages = [{ path: '/admin/members', label: 'Members & invites' }]
