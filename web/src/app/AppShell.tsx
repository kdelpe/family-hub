import * as Dialog from '@radix-ui/react-dialog'
import * as Tooltip from '@radix-ui/react-tooltip'
import { useQuery } from '@tanstack/react-query'
import { m } from 'motion/react'
import { useState } from 'react'
import { Link, Navigate, NavLink, Outlet, useLocation } from 'react-router'

import { ApiError } from '../api/client'
import { myRequestsQuery } from '../api/contacts'
import { Logo } from '../components/Logo/Logo'
import { initials } from '../lib/names'
import { errorMessage } from '../lib/errorMessage'
import { useNarrow } from '../lib/useNarrow'
import { useSignedIn } from '../lib/useSignedIn'
import { AccountMenu } from './AccountMenu'
import sheet from './AccountMenu.module.css'
import styles from './AppShell.module.css'
import { MenuIcon } from './MenuIcon'
import { adminPages, menuPages } from './signedInPages'


const foldedKey = 'family-hub.menu-folded'

// The menu folded to its rail stays folded on the next visit. Storage can be missing or refused.
function useFolded() {
  const [folded, setFolded] = useState(() => {
    try {
      return localStorage.getItem(foldedKey) === '1'
    } catch {
      return false
    }
  })
  const fold = (next: boolean) => {
    setFolded(next)
    try {
      localStorage.setItem(foldedKey, next ? '1' : '0')
    } catch {
      // Folded for this visit only.
    }
  }
  return [folded, fold] as const
}

// One highlight that glides to the page opened. On a phone, short is the tab's label.
function MenuItem({ path, label, short, icon, rail, narrow }: (typeof menuPages)[number] & { rail: boolean; narrow: boolean }) {
  const tab = narrow && short
  const link = (
    // NavLink marks the current page with aria-current, which the styles read: a plain className, which the tooltip's
    // trigger can merge with its own.
    <NavLink to={path} end aria-label={tab ? label : undefined} className={styles.item}>
      {({ isActive }) => (
        <>
          {isActive && <m.span layoutId="menu-current" className={styles.currentPill} transition={{ type: 'spring', stiffness: 380, damping: 32 }} />}
          <MenuIcon name={icon} className={styles.icon} />
          <span className={styles.label}>{tab ? short : label}</span>
        </>
      )}
    </NavLink>
  )
  if (!rail) {
    return link
  }
  return (
    <Tooltip.Root>
      <Tooltip.Trigger asChild>{link}</Tooltip.Trigger>
      <Tooltip.Portal>
        <Tooltip.Content side="right" sideOffset={10} className={styles.tip}>
          {label}
        </Tooltip.Content>
      </Tooltip.Portal>
    </Tooltip.Root>
  )
}

// Phones: the admin pages don't fit the tab bar, so admins get them in a drawer from the top right, the account
// panel's sheet.
function AdminDrawer() {
  const [open, setOpen] = useState(false)
  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger aria-label="Admin menu" className={styles.drawerButton}>
        <span aria-hidden="true" className={styles.bars} />
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className={sheet.scrim} />
        <Dialog.Content aria-describedby={undefined} className={`${sheet.panel} ${styles.drawer}`}>
          <Dialog.Title className={styles.drawerTitle}>Admin</Dialog.Title>
          <nav aria-label="Admin">
            {adminPages.map(({ path, label, icon }) => (
              <NavLink key={path} to={path} end onClick={() => setOpen(false)} className={styles.item}>
                <MenuIcon name={icon} className={styles.icon} />
                <span className={styles.label}>{label}</span>
              </NavLink>
            ))}
          </nav>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

// The parent route of every signed-in page (design-notes §2.1): the left menu, then the page's own <main>.
// Signed out, it sends you to sign in, and back here after.
export function AppShell() {
  const { user, person, personSettled, error } = useSignedIn()
  const location = useLocation()
  // The account picture's count: requests for your details waiting for an answer (§5.2). Only a linked login gets any.
  // Reread each minute, so a new request shows without a reload; not while the tab is hidden.
  const personId = user?.person_id ?? null
  const requests = useQuery({ ...myRequestsQuery(personId ?? ''), enabled: personId !== null, refetchInterval: 60_000 })
  const waiting = requests.data?.filter((r) => r.status === 'pending').length ?? 0
  const [folded, fold] = useFolded()
  const narrow = useNarrow()
  // Phones have the tab bar instead, whatever was folded on a wider screen.
  const rail = folded && !narrow

  if (error instanceof ApiError && error.status === 401) {
    return <Navigate to="/auth/login" replace state={{ from: location.pathname + location.search }} />
  }
  // A failed refetch keeps the answer it had, so a blip doesn't take the page away.
  if (error && !user) {
    return (
      <main className={styles.failed}>
        <p role="alert">{errorMessage(error)}</p>
      </main>
    )
  }
  // Waits for the name too, so "Signed in as" doesn't flash the email first.
  if (!user || !personSettled) {
    return null
  }

  return (
    <div className={rail ? `${styles.page} ${styles.folded}` : styles.page}>
      <nav aria-label="Main" className={styles.menu}>
        <button type="button" aria-expanded={!folded} aria-label={folded ? 'Unfold the menu' : 'Fold the menu'} onClick={() => fold(!folded)} className={styles.fold}>
          <span aria-hidden="true">‹</span>
        </button>
        <Link to="/" aria-label="Pluviose Family Hub, home" className={styles.identity}>
          <Logo size={52} />
          <span className={styles.names}>
            <span className={styles.familyName}>Pluviose</span>
            <span className={styles.hub}>Family Hub</span>
          </span>
        </Link>
        <Tooltip.Provider delayDuration={200}>
          <div className={styles.items}>
            {menuPages.map((page) => (
              <MenuItem key={page.path} {...page} rail={rail} narrow={narrow} />
            ))}
          </div>
          {user.role === 'admin' && (
            <div className={`${styles.items} ${styles.admin}`}>
              <span className={styles.adminLabel}>Admin</span>
              {adminPages.map((page) => (
                <MenuItem key={page.path} {...page} rail={rail} narrow={narrow} />
              ))}
            </div>
          )}
        </Tooltip.Provider>
      </nav>
      <div className={styles.content}>
        <div className={styles.account}>
          {/* Phones only: the menu's top is gone to the tab bar, so the name and home link sit here. */}
          <Link to="/" aria-label="Pluviose Family Hub, home" className={styles.phoneIdentity}>
            <Logo size={34} />
            <span className={styles.familyName}>Pluviose</span>
          </Link>
          {/* Only while it can show: a drawer left open as the screen widens would have no ☰ to close it. */}
          {user.role === 'admin' && narrow && <AdminDrawer />}
          <AccountMenu
            initials={person ? initials(person) : (user.email[0] ?? '?').toUpperCase()}
            name={person ? `${person.first_name} ${person.last_name ?? ''}`.trim() : user.email}
            waiting={waiting}
            linked={person !== null && person !== undefined}
          />
        </div>
        <Outlet />
      </div>
    </div>
  )
}
