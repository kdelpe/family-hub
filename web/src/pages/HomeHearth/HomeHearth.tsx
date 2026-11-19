import { Link } from 'react-router'

import { dayPartLabel, greeting } from '../../lib/dayPart'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './HomeHearth.module.css'

// Home (design-notes §4.1). For now only the hero's words and buttons: its personal line, the small tree and
// the cards below need birthdays, photos and stories the API doesn't serve yet.
export function HomeHearth({ now = new Date() }: { now?: Date }) {
  const { person } = useSignedIn()
  const hello = greeting(now)

  return (
    <main className={styles.page}>
      <title>Home — Pluviose-Monestime Family Hub</title>
      <section aria-label="Welcome" className={styles.hero}>
        <div className={styles.words}>
          <span className={styles.dayPart}>{dayPartLabel(now)}</span>
          <h1 className={styles.title}>{person ? `${hello}, ${person.first_name}` : hello}</h1>
          <div className={styles.actions}>
            <Link to="/tree" className={`pill ${styles.primary}`}>
              Open the family tree
            </Link>
            <Link to="/events" className={`pill ${styles.secondary}`}>
              See new photos
            </Link>
          </div>
        </div>
      </section>
    </main>
  )
}
