import { Link, Navigate, NavLink, Outlet, useLocation } from 'react-router'

import { ApiError } from '../api/client'
import { Logo } from '../components/Logo/Logo'
import { errorMessage } from '../lib/errorMessage'
import { useSignedIn } from '../lib/useSignedIn'
import styles from './AppShell.module.css'
import { adminPages, menuPages } from './signedInPages'

const item = ({ isActive }: { isActive: boolean }) => (isActive ? `${styles.item} ${styles.current}` : styles.item)

// The parent route of every signed-in page (design-notes §2.1): the left menu, then the page's own <main>.
// Signed out, it sends you to sign in, and back here after.
export function AppShell() {
  const { user, person, personSettled, error } = useSignedIn()
  const location = useLocation()

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
        <Link to="/" aria-label="Pluviose-Monestime Family Hub, home" className={styles.identity}>
          <Logo size={52} />
          <span className={styles.names}>
            <span className={styles.familyName}>Pluviose-Monestime</span>
            <span className={styles.hub}>Family Hub</span>
          </span>
        </Link>
        <div className={styles.items}>
          {menuPages.map(({ path, label }) => (
            <NavLink key={path} to={path} end className={item}>
              {label}
            </NavLink>
          ))}
        </div>
        {user.role === 'admin' && (
          <div className={`${styles.items} ${styles.admin}`}>
            <span className={styles.adminLabel}>Admin</span>
            {adminPages.map(({ path, label }) => (
              <NavLink key={path} to={path} end className={item}>
                {label}
              </NavLink>
            ))}
          </div>
        )}
        <div className={styles.signedIn}>Signed in as {person?.first_name ?? user.email}</div>
      </nav>
      <div className={styles.content}>
        <Outlet />
      </div>
    </div>
  )
}
