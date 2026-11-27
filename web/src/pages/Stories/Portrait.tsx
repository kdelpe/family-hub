import type { StoryPerson } from '../../api/stories'
import { initials } from '../../lib/names'
import styles from './Portrait.module.css'
import { pick } from './storyText'

const tones = [styles.t1, styles.t2, styles.t3, styles.t4, styles.t5]

// A storyteller's initials in a gold ring, in a colour of their own.
export function Portrait({ person, small = false, className }: { person: StoryPerson; small?: boolean; className?: string }) {
  return (
    <span aria-hidden="true" className={[styles.portrait, small && styles.small, pick(tones, person.id), className].filter(Boolean).join(' ')}>
      {initials(person)}
    </span>
  )
}
