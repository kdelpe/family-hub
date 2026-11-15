import { useState } from 'react'

import styles from './PasswordInput.module.css'

type Props = {
  id: string
  autoComplete: 'current-password' | 'new-password'
  value: string
  onChange: (value: string) => void
  readOnly?: boolean
  initiallyShown?: boolean
  describedBy?: string
  invalid?: boolean
}

// The label stays with the page.
export function PasswordInput({ id, autoComplete, value, onChange, readOnly, initiallyShown = false, describedBy, invalid }: Props) {
  const [shown, setShown] = useState(initiallyShown)
  return (
    <div className={styles.box}>
      <input
        id={id}
        type={shown ? 'text' : 'password'}
        autoComplete={autoComplete}
        required
        readOnly={readOnly}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        className={styles.input}
      />
      <button type="button" aria-pressed={shown} onClick={() => setShown(!shown)} className={styles.reveal}>
        {shown ? 'Hide' : 'Show'}
      </button>
    </div>
  )
}
