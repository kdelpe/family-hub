import { type KeyboardEvent, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router'

import { errorMessage } from '../lib/errorMessage'
import { useSignInBusy, useSignOut } from '../lib/useSignOut'
import styles from './AccountMenu.module.css'

// design-notes §2.1: your picture at the top right, opening a panel from the right (owner's pick A, 2026-11-21).
export function AccountMenu({ initials, name, waiting, linked }: { initials: string; name: string; waiting: number; linked: boolean }) {
  const [open, setOpen] = useState(false)
  const button = useRef<HTMLButtonElement>(null)
  const panel = useRef<HTMLDivElement>(null)
  const signOut = useSignOut()
  const busy = useSignInBusy()

  useEffect(() => {
    if (open) {
      panel.current?.querySelector<HTMLElement>('a, button')?.focus()
    }
  }, [open])
  const close = () => {
    setOpen(false)
    button.current?.focus()
  }
  // Esc closes it; Tab stays inside while it's open.
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      close()
    } else if (event.key === 'Tab') {
      const items = [...(panel.current?.querySelectorAll<HTMLElement>('a, button') ?? [])]
      const edge = event.shiftKey ? items[0] : items[items.length - 1]
      if (document.activeElement === edge) {
        event.preventDefault()
        ;(event.shiftKey ? items[items.length - 1] : items[0])?.focus()
      }
    }
  }
  const requests = `${waiting} ${waiting === 1 ? 'request' : 'requests'} waiting`

  return (
    <>
      <button
        ref={button}
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={waiting > 0 ? `Your account, ${requests}` : 'Your account'}
        onClick={() => setOpen(true)}
        className={styles.picture}
      >
        {initials}
        {waiting > 0 && (
          <span aria-hidden="true" className={styles.dot}>
            {waiting}
          </span>
        )}
      </button>
      {/* On the page's body, above every page's own overlays (the tree's search results, the profile's undo note). */}
      {open &&
        createPortal(
        <>
          <div className={styles.scrim} onClick={close} />
          <div ref={panel} role="dialog" aria-modal="true" aria-label="Your account" onKeyDown={onKeyDown} className={styles.panel}>
            <div className={styles.who}>
              <span aria-hidden="true" className={`${styles.picture} ${styles.large}`}>
                {initials}
              </span>
              <span>
                <span className={styles.name}>{name}</span>
                <Link to="/profile" onClick={() => setOpen(false)} className={styles.profile}>
                  View my profile ›
                </Link>
              </span>
            </div>
            <Link to="/requests" onClick={() => setOpen(false)} className={styles.item} aria-label={waiting > 0 ? `Requests, ${requests}` : undefined}>
              <span>
                Requests
                <span className={styles.hint}>Family asking for your details</span>
              </span>
              {waiting > 0 && (
                <span aria-hidden="true" className={styles.count}>
                  {waiting}
                </span>
              )}
            </Link>
            {/* A login linked to nobody can't suggest anyone, as on the Account page. */}
            {linked && (
              <Link to="/account#suggest" onClick={() => setOpen(false)} className={styles.item}>
                Suggest a relative
              </Link>
            )}
            <Link to="/account#password" onClick={() => setOpen(false)} className={styles.item}>
              Change password
            </Link>
            <hr className={styles.rule} />
            <button type="button" onClick={() => !busy && signOut.mutate()} aria-disabled={busy || undefined} className={styles.item}>
              Sign out
            </button>
            {signOut.isError && <p role="alert">{errorMessage(signOut.error)}</p>}
          </div>
        </>,
          document.body,
        )}
    </>
  )
}
