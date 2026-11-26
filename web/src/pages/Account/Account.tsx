import { useEffect } from 'react'
import { useLocation } from 'react-router'

import { fullName, initials } from '../../lib/names'
import { useSignedIn } from '../../lib/useSignedIn'
import { SignInCard } from './SignInCard'
import { SuggestCard } from './SuggestCard'
import styles from './Account.module.css'

// Opened from the account menu (design-notes §2.1): changing your password, and suggesting a relative.
export function Account() {
  const { user, person } = useSignedIn()
  const { hash } = useLocation()
  useEffect(() => {
    document.getElementById(hash.slice(1))?.scrollIntoView({ block: 'start' })
  }, [hash])
  if (!user) {
    return null
  }
  return (
    <main className={styles.page}>
      <title>Account — Pluviose Family Hub</title>
      <h1 className={styles.title}>Account</h1>
      {/* Who's signed in, as at the top of the account menu. */}
      <div className={styles.who}>
        <span aria-hidden="true" className={styles.picture}>
          {person ? initials(person) : (user.email[0] ?? '?').toUpperCase()}
        </span>
        <span>
          {person && <span className={styles.name}>{fullName(person)}</span>}
          <span className={styles.email}>{user.email}</span>
        </span>
      </div>
      <div id="password" className={styles.card}>
        <SignInCard email={user.email} />
      </div>
      {person && (
        <div id="suggest" className={styles.card}>
          <SuggestCard key={person.id} personId={person.id} yourName={fullName(person)} />
        </div>
      )}
    </main>
  )
}
