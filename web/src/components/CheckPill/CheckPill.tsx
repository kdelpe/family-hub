import type { ReactNode } from 'react'
import { Link } from 'react-router'

import styles from './CheckPill.module.css'

// The tinted "✓ …" pill for something already done, like "Phone number requested" (design-notes §4.4).
export function CheckPill({ size, to, children }: { size: 'large' | 'small'; to?: string; children: ReactNode }) {
  const className = `${styles.pill} ${size === 'large' ? styles.large : styles.small}`
  const content = (
    <>
      <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24">
        <path d="M5 12.5 L10 17 L19 7" />
      </svg>
      {children}
    </>
  )
  return to ? (
    <Link to={to} className={`${className} ${styles.link}${size === 'small' ? ' tap' : ''}`}>
      {content}
    </Link>
  ) : (
    <span className={className}>{content}</span>
  )
}
