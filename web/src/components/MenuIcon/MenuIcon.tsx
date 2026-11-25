import type { ReactNode } from 'react'

import { useMotionPaused } from '../../lib/motion'
import styles from './MenuIcon.module.css'

export type MenuIconName = 'home' | 'tree' | 'book' | 'calendar' | 'mic' | 'key' | 'inbox' | 'addPerson' | 'lock' | 'signOut' | 'search' | 'crosshair'

const paths: Record<MenuIconName, ReactNode> = {
  home: (
    <>
      <path d="M3.5 11 12 4l8.5 7" />
      <path d="M5.5 9.5V20h13V9.5" />
      <path className={styles.door} d="M10 20v-5.5h4V20" />
    </>
  ),
  tree: (
    <>
      <path d="M12 21v-8M12 15l-4-3M12 13l4-3" />
      <circle cx="7" cy="10.5" r="2.3" />
      <circle cx="17" cy="8.5" r="2.3" />
      <circle cx="12" cy="6" r="2.6" />
    </>
  ),
  book: (
    <>
      <path d="M12 6.5c-2-1.5-5-2-8-1.5v13c3-.5 6 0 8 1.5" />
      <path className={styles.cover} d="M12 6.5c2-1.5 5-2 8-1.5v13c-3-.5-6 0-8 1.5z" />
    </>
  ),
  calendar: (
    <>
      <rect x="4" y="5.5" width="16" height="14.5" rx="2.5" />
      <path d="M8 3.5v4M16 3.5v4M8.5 14h2M13.5 14h2M8.5 17h2" />
      <path className={styles.page} d="M4 10h16" />
    </>
  ),
  mic: (
    <>
      <rect x="9" y="3.5" width="6" height="11" rx="3" />
      <path d="M6 11.5a6 6 0 0 0 12 0M12 17.5v3" />
    </>
  ),
  key: (
    <>
      <circle cx="8" cy="12" r="3.5" />
      <path d="M11.5 12H20l-2 2M16.5 12v2.5" />
    </>
  ),
  inbox: (
    <>
      <path d="M4 13.5 6.5 5h11l2.5 8.5V19H4z" />
      <path d="M4 13.5h5l1 2h4l1-2h5" />
    </>
  ),
  addPerson: (
    <>
      <circle cx="10" cy="8.5" r="3.5" />
      <path d="M3.5 19c.8-3.4 3.4-5 6.5-5s5.7 1.6 6.5 5M18.5 7v6M15.5 10h6" />
    </>
  ),
  lock: (
    <>
      <rect x="5" y="10.5" width="14" height="9.5" rx="2.5" />
      <path d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5" />
    </>
  ),
  signOut: <path d="M14 4.5H6.5v15H14M10.5 12H20M17 8.5l3.5 3.5-3.5 3.5" />,
  search: (
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m20 20-4.2-4.2" />
    </>
  ),
  crosshair: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2v4M12 18v4M2 12h4M18 12h4" />
    </>
  ),
}

// The line icons of the menu, account sheet and tree toolbar (design-notes §2.1), drawn in the text's colour. The
// menu's own icons move when their link is pointed at or focused (MenuIcon.module.css).
export function MenuIcon({ name, className }: { name: MenuIconName; className?: string }) {
  const [paused] = useMotionPaused()
  return (
    <svg
      aria-hidden="true"
      data-icon={name}
      data-still={paused || undefined}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={`${styles.icon} ${className ?? ''}`}
    >
      {paths[name]}
    </svg>
  )
}
