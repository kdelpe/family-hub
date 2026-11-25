import { useQuery } from '@tanstack/react-query'
import { m } from 'motion/react'
import { Link, Navigate, NavLink, Outlet, useLocation } from 'react-router'

import { ApiError } from '../api/client'
import { myRequestsQuery } from '../api/contacts'
import { Logo } from '../components/Logo/Logo'
import { initials } from '../lib/names'
import { errorMessage } from '../lib/errorMessage'
import { useSignedIn } from '../lib/useSignedIn'
import { AccountMenu } from './AccountMenu'
import styles from './AppShell.module.css'
import { adminPages, menuPages } from './signedInPages'

const item = ({ isActive }: { isActive: boolean }) => (isActive ? `${styles.item} ${styles.current}` : styles.item)

// One highlight that glides to the page opened.
function MenuLabel({ label, current }: { label: string; current: boolean }) {
  return (
    <>
      {current && <m.span layoutId="menu-current" className={styles.currentPill} transition={{ type: 'spring', stiffness: 380, damping: 32 }} />}
      <span className={styles.label}>{label}</span>
    </>
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
    <div className={styles.page}>
      <nav aria-label="Main" className={styles.menu}>
        <Link to="/" aria-label="Pluviose Family Hub, home" className={styles.identity}>
          <Logo size={52} />
          <span className={styles.names}>
            <span className={styles.familyName}>Pluviose</span>
            <span className={styles.hub}>Family Hub</span>
          </span>
        </Link>
        <div className={styles.items}>
          {menuPages.map(({ path, label }) => (
            <NavLink key={path} to={path} end className={item}>
              {({ isActive }) => <MenuLabel label={label} current={isActive} />}
            </NavLink>
          ))}
        </div>
        {user.role === 'admin' && (
          <div className={`${styles.items} ${styles.admin}`}>
            <span className={styles.adminLabel}>Admin</span>
            {adminPages.map(({ path, label }) => (
              <NavLink key={path} to={path} end className={item}>
                {({ isActive }) => <MenuLabel label={label} current={isActive} />}
              </NavLink>
            ))}
          </div>
        )}
      </nav>
      <div className={styles.content}>
        <div className={styles.account}>
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
