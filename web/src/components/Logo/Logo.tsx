import styles from './Logo.module.css'

// The placeholder mark (design-notes §2.2), drawn at whatever size the page gives it.
export function Logo({ size }: { size: number }) {
  return (
    <svg aria-hidden="true" width={size} height={size} viewBox="0 0 52 52">
      <circle cx="26" cy="26" r="25" className={styles.ground} strokeWidth="1.5" />
      <path d="M26 42 V24 M26 30 L17 22 M26 30 L35 22 M26 24 L26 15" className={styles.stem} strokeWidth="2.4" />
      <circle cx="26" cy="13" r="4" className={styles.top} />
      <circle cx="16" cy="20" r="3.4" className={styles.leaf} />
      <circle cx="36" cy="20" r="3.4" className={styles.leaf} />
    </svg>
  )
}
