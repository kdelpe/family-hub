import * as Dialog from '@radix-ui/react-dialog'
import * as Tooltip from '@radix-ui/react-tooltip'
import { useQuery } from '@tanstack/react-query'
import { m } from 'motion/react'
import { useEffect, useState } from 'react'
import { Link, Navigate, NavLink, Outlet, useLocation } from 'react-router'

import { ApiError } from '../api/client'
import { myRequestsQuery } from '../api/contacts'
import { Logo } from '../components/Logo/Logo'
import { initials } from '../lib/names'
import { errorMessage } from '../lib/errorMessage'
import { useMenuFolded } from '../lib/motion'
import { useNarrow } from '../lib/useNarrow'
import { useSignedIn } from '../lib/useSignedIn'
import { AccountMenu } from './AccountMenu'
import sheet from './AccountMenu.module.css'
import styles from './AppShell.module.css'
import { MenuIcon } from './MenuIcon'
import { adminPages, menuPages } from './signedInPages'


// One highlight that glides to the page opened. On a phone, short is the tab's label.
function MenuItem({
  path,
  label,
  short,
  icon,
  rail,
  narrow,
  onClick,
}: (typeof menuPages)[number] & { rail: boolean; narrow: boolean; onClick?: () => void }) {
  const tab = narrow && short
  const [tipOpen, setTipOpen] = useState(false)
  // Always inside the tooltip, closed off the rail: unwrapping it would remount the link and drop its focus. NavLink
  // marks the current page with aria-current, which the styles read, so its className stays plain for the trigger.
  return (
    <Tooltip.Root open={rail && tipOpen} onOpenChange={setTipOpen}>
      <Tooltip.Trigger asChild>
        <NavLink to={path} end aria-label={tab ? label : undefined} onClick={onClick} className={styles.item}>
          {({ isActive }) => (
            <>
              {isActive && <m.span layoutId="menu-current" className={styles.currentPill} transition={{ type: 'spring', stiffness: 380, damping: 32 }} />}
              <MenuIcon name={icon} className={styles.icon} />
              <span className={styles.label}>{tab ? short : label}</span>
            </>
          )}
        </NavLink>
      </Tooltip.Trigger>
      <Tooltip.Portal>
        <Tooltip.Content side="right" sideOffset={10} className={styles.tip}>
          {label}
        </Tooltip.Content>
      </Tooltip.Portal>
    </Tooltip.Root>
  )
}

// Phones: the admin pages don't fit the tab bar, so a ☰ at the top opens them in a drawer from the right, the
// account panel's sheet.
function AdminDrawer() {
  const [open, setOpen] = useState(false)
  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger aria-label="Admin menu" className={styles.drawerButton}>
        <span aria-hidden="true" className={styles.bars} />
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className={sheet.scrim} />
        {/* Focus on the sheet, not its first link, whose icon would otherwise start moving on its own. */}
        <Dialog.Content
          aria-describedby={undefined}
          onOpenAutoFocus={(event) => {
            event.preventDefault()
            ;(event.currentTarget as HTMLElement).focus()
          }}
          className={`${sheet.panel} ${styles.drawer}`}
        >
          <Dialog.Title className={styles.drawerTitle}>Admin</Dialog.Title>
          <nav aria-label="Admin">
            {adminPages.map((page) => (
              <MenuItem key={page.path} {...page} rail={false} narrow={false} onClick={() => setOpen(false)} />
            ))}
          </nav>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

// Toasts live outside the shell, so the page root says when the phone's tab bar is there to keep clear of.
function useTabBarRoom(shown: boolean) {
  useEffect(() => {
    if (!shown) {
      return
    }
    document.documentElement.toggleAttribute('data-tab-bar', true)
    return () => document.documentElement.removeAttribute('data-tab-bar')
  }, [shown])
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
  const [folded, fold] = useMenuFolded()
  const narrow = useNarrow()
  // Phones have the tab bar instead, whatever was folded on a wider screen.
  const rail = folded && !narrow
  // Not while loading or failed, which show no menu.
  useTabBarRoom(Boolean(user && personSettled))

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
    <Tooltip.Provider delayDuration={200}>
      <div className={rail ? `${styles.page} ${styles.folded}` : styles.page}>
        <nav id="main-menu" aria-label="Main" className={styles.menu}>
          <button type="button" aria-controls="main-menu" aria-expanded={!folded} aria-label={folded ? 'Unfold the menu' : 'Fold the menu'} onClick={() => fold(!folded)} className={styles.fold}>
            <span aria-hidden="true">‹</span>
          </button>
          <Link to="/" aria-label="Pluviose Family Hub, home" className={styles.identity}>
            <Logo size={52} />
            <span className={styles.names}>
              <span className={styles.familyName}>Pluviose</span>
              <span className={styles.hub}>Family Hub</span>
            </span>
          </Link>
          <div className={styles.items}>
            {menuPages.map((page) => (
              <MenuItem key={page.path} {...page} rail={rail} narrow={narrow} />
            ))}
          </div>
          {/* On a phone these live in the drawer instead; both at once would share the current-page pill. */}
          {user.role === 'admin' && !narrow && (
            <div className={`${styles.items} ${styles.admin}`}>
              <span className={styles.adminLabel}>Admin</span>
              {adminPages.map((page) => (
                <MenuItem key={page.path} {...page} rail={rail} narrow={narrow} />
              ))}
            </div>
          )}
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
    </Tooltip.Provider>
  )
}
