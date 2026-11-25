import { type KeyboardEvent, useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router'

import sheet from '../components/Sheet/Sheet.module.css'
import { errorMessage } from '../lib/errorMessage'
import { useMotionPaused } from '../lib/motion'
import { useSignInBusy, useSignOut } from '../lib/useSignOut'
import styles from './AccountMenu.module.css'
import { MenuIcon } from './MenuIcon'

// design-notes §2.1: your picture at the top right, opening a floating sheet on the right (owner's pick C, 2026-11-25).
export function AccountMenu({ initials, name, waiting, linked }: { initials: string; name: string; waiting: number; linked: boolean }) {
  const [open, setOpen] = useState(false)
  const [paused] = useMotionPaused()
  const familyId = useId()
  const accountId = useId()
  const button = useRef<HTMLButtonElement>(null)
  const panel = useRef<HTMLDivElement>(null)
  const signOut = useSignOut()
  const busy = useSignInBusy()

  useEffect(() => {
    if (!open) {
      return
    }
    // The rest of the app can't take focus or clicks while it's open, so nothing behind the scrim is reached.
    const app = document.getElementById('root')
    if (app) {
      app.inert = true
    }
    panel.current?.querySelector<HTMLElement>('a, button')?.focus()
    return () => {
      if (app) {
        app.inert = false
      }
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
          <div data-still={paused || undefined} className={sheet.scrim} onClick={close} />
          <div ref={panel} role="dialog" aria-modal="true" aria-label="Your account" onKeyDown={onKeyDown} data-still={paused || undefined} className={`${sheet.panel} ${styles.panel}`}>
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
            <p id={familyId} className={styles.groupLabel}>
              Family
            </p>
            <div role="group" aria-labelledby={familyId} className={styles.group}>
              <Link to="/requests" onClick={() => setOpen(false)} className={styles.item} aria-label={waiting > 0 ? `Requests, ${requests}` : undefined}>
                <MenuIcon name="inbox" className={styles.icon} />
                <span className={styles.label}>Requests</span>
                {waiting > 0 && (
                  <span aria-hidden="true" className={styles.count}>
                    {waiting}
                  </span>
                )}
                <span aria-hidden="true" className={styles.chevron}>
                  ›
                </span>
              </Link>
              {/* A login linked to nobody can't suggest anyone, as on the Account page. */}
              {linked && (
                <Link to="/account#suggest" onClick={() => setOpen(false)} className={styles.item}>
                  <MenuIcon name="addPerson" className={styles.icon} />
                  <span className={styles.label}>Suggest a relative</span>
                  <span aria-hidden="true" className={styles.chevron}>
                    ›
                  </span>
                </Link>
              )}
            </div>
            <p id={accountId} className={styles.groupLabel}>
              Account
            </p>
            <div role="group" aria-labelledby={accountId} className={styles.group}>
              <Link to="/account#password" onClick={() => setOpen(false)} className={styles.item}>
                <MenuIcon name="lock" className={styles.icon} />
                <span className={styles.label}>Change password</span>
                <span aria-hidden="true" className={styles.chevron}>
                  ›
                </span>
              </Link>
            </div>
            <div className={`${styles.group} ${styles.foot}`}>
              <button type="button" onClick={() => !busy && signOut.mutate()} aria-disabled={busy || undefined} className={`${styles.item} ${styles.signOut}`}>
                <MenuIcon name="signOut" className={styles.icon} />
                <span className={styles.label}>Sign out</span>
              </button>
            </div>
            {signOut.isError && <p role="alert">{errorMessage(signOut.error)}</p>}
          </div>
        </>,
          document.body,
        )}
    </>
  )
}
