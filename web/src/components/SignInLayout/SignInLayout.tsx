import { Outlet } from 'react-router'

import { Logo } from '../Logo/Logo'
import { SignInTree } from '../SignInTree/SignInTree'
import styles from './SignInLayout.module.css'

// The parent route of every page with the brand panel (sign-in pages and First setup), then the page's own <main>.
// As a layout route it stays mounted between those pages, so the tree doesn't regrow on each move.
export function SignInLayout() {
  return (
    <div className={styles.page}>
      <section aria-label="About this site" className={styles.brand}>
        <div className={styles.identity}>
          <Logo size={64} />
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
