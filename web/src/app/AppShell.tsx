import { Link, Navigate, NavLink, Outlet } from 'react-router'

import { ApiError } from '../api/client'
import { Logo } from '../components/Logo/Logo'
import { errorMessage } from '../lib/errorMessage'
import { useSignedIn } from '../lib/useSignedIn'
import styles from './AppShell.module.css'

const pages = [
  { to: '/', label: 'Home' },
  { to: '/tree', label: 'Family tree' },
  { to: '/people', label: 'People' },
  { to: '/directory', label: 'Directory' },
  { to: '/events', label: 'Events' },
  { to: '/stories', label: 'Stories' },
  { to: '/profile', label: 'My profile' },
]

const item = ({ isActive }: { isActive: boolean }) => (isActive ? `${styles.item} ${styles.current}` : styles.item)

// The parent route of every signed-in page (design-notes §2.1): the left menu, then the page's own <main>.
// Signed out, it sends you to sign in.
export function AppShell() {
  const { user, person, personSettled, error } = useSignedIn()

  if (error instanceof ApiError && error.status === 401) {
    return <Navigate to="/auth/login" replace />
  }
  if (error) {
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
          {pages.map(({ to, label }) => (
            <NavLink key={to} to={to} end className={item}>
              {label}
            </NavLink>
          ))}
        </div>
        {user.role === 'admin' && (
          <div className={`${styles.items} ${styles.admin}`}>
            <span className={styles.adminLabel}>Admin</span>
            <NavLink to="/admin/members" end className={item}>
              Members &amp; invites
            </NavLink>
          </div>
        )}
        <div className={styles.signedIn}>Signed in as {person?.first_name ?? user.email}</div>
      </nav>
      <Outlet />
    </div>
  )
}
