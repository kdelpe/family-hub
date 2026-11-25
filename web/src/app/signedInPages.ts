import type { MenuIconName } from './MenuIcon'

export type MenuPage = { path: string; label: string; icon: MenuIconName; short?: string }

// The signed-in pages, in menu order (design-notes §2.1). The menu and the routes both read this list, so a
// menu link always lands inside the shell. short: the phone's tab label, where the label won't fit.
export const menuPages: MenuPage[] = [
  { path: '/', label: 'Home', icon: 'home' },
  { path: '/tree', label: 'Family tree', icon: 'tree', short: 'Tree' },
  { path: '/members', label: 'Members', icon: 'book' },
  { path: '/events', label: 'Events', icon: 'calendar' },
  { path: '/stories', label: 'Stories', icon: 'mic' },
]

export const adminPages: MenuPage[] = [{ path: '/admin/members', label: 'Members & invites', icon: 'key' }]
