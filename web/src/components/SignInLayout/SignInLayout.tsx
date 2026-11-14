import { Outlet } from 'react-router'

import { SignInTree } from '../SignInTree/SignInTree'
import styles from './SignInLayout.module.css'

// The parent route of every page before sign-in: the brand panel, then the page's own <main>. As a
// layout route it stays mounted between those pages, so the tree doesn't regrow on each move.
export function SignInLayout() {
  return (
    <div className={styles.page}>
      <section aria-label="About this site" className={styles.brand}>
        <div className={styles.identity}>
          <svg aria-hidden="true" width="64" height="64" viewBox="0 0 52 52">
            <circle cx="26" cy="26" r="25" className={styles.logoGround} strokeWidth="1.5" />
            <path d="M26 42 V24 M26 30 L17 22 M26 30 L35 22 M26 24 L26 15" className={styles.logoStem} strokeWidth="2.4" />
            <circle cx="26" cy="13" r="4" className={styles.logoTop} />
            <circle cx="16" cy="20" r="3.4" className={styles.logoLeaf} />
            <circle cx="36" cy="20" r="3.4" className={styles.logoLeaf} />
          </svg>
          <span className={styles.names}>
            <span className={styles.familyName}>Pluviose-Monestime</span>
            <span className={styles.hub}>Family Hub</span>
          </span>
        </div>
        <SignInTree />
        <div className={styles.words}>
          <p className={styles.tagline}>
            A private home for our family’s stories,
            <br />
            photos and history.
          </p>
          <p className={styles.proverb}>
            <span lang="ht">“Sa w plante, se li w rekòlte.”</span> — What you plant is what you harvest.
          </p>
        </div>
      </section>
      <Outlet />
    </div>
  )
}
