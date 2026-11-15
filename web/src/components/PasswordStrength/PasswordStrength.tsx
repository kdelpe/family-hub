import { passwordStrength } from '../../lib/passwordStrength'
import styles from './PasswordStrength.module.css'

const segments = [1, 2, 3, 4] as const

export function PasswordStrength({ password }: { password: string }) {
  const { level, label } = passwordStrength(password)
  return (
    <div className={styles.meter}>
      <div aria-hidden="true" className={styles.bars}>
        {segments.map((segment) => (
          <span key={segment} className={segment <= level ? `${styles.bar} ${styles.filled}` : styles.bar} />
        ))}
      </div>
      <span aria-live="polite" aria-atomic="true" className={level === 1 ? `${styles.label} ${styles.short}` : styles.label}>
        {label && <span className="visually-hidden">Password strength: </span>}
        {label}
      </span>
    </div>
  )
}
