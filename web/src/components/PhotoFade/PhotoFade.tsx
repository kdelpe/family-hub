import { fullName } from '../../lib/names'
import styles from './PhotoFade.module.css'

const tones = [styles.tone0, styles.tone1, styles.tone2, styles.tone3, styles.tone4, styles.tone5]

// Until photos arrive, each portrait fades now and then to a placeholder silhouette, at its own pace, as in
// the mockups. Sits inside a round portrait that shows the initials; paused, it stays on the initials.
export function PhotoFade({ person, paused }: { person: { first_name: string; last_name: string | null }; paused: boolean }) {
  const h = [...fullName(person)].reduce((sum, c) => sum + c.charCodeAt(0), 0)
  return (
    <span
      aria-hidden="true"
      className={`${styles.photo} ${tones[h % tones.length]} ${paused ? styles.paused : ''}`}
      style={{ animationDelay: `${((h % 11) * 0.6 + 0.8).toFixed(1)}s` }}
    >
      <span className={styles.head} />
      <span className={styles.shoulders} />
    </span>
  )
}
