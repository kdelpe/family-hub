import { useEffect } from 'react'
import { useLocation } from 'react-router'

import { fullName } from '../../lib/names'
import { useSignedIn } from '../../lib/useSignedIn'
import { SignInCard } from '../MyProfile/SignInCard'
import { SuggestCard } from '../MyProfile/SuggestCard'
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
