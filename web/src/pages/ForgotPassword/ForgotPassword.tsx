import { useMutation } from '@tanstack/react-query'
import { type FormEvent, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'

import { forgotPassword } from '../../api/auth'
import { errorMessage } from '../../lib/errorMessage'
import page from '../../styles/signInForm.module.css'
import styles from './ForgotPassword.module.css'

export function ForgotPassword() {
  const [email, setEmail] = useState('')
  const send = useMutation({ mutationFn: () => forgotPassword(email) })
  const sentHeading = useRef<HTMLHeadingElement>(null)

  // The focused button goes away with the form; the heading takes focus, so keyboard users land on the result.
  useEffect(() => {
    if (send.isSuccess) {
      sentHeading.current?.focus()
    }
  }, [send.isSuccess])

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!send.isPending) {
      send.mutate()
    }
  }

  return (
    <>
      <title>Forgot password — Pluviose-Monestime Family Hub</title>
      <main className={page.main}>
        <div className={page.form}>
          <Link to="/auth/login" className={page.backLink}>
            <span aria-hidden="true">← </span>Back to sign in
          </Link>
          {send.isSuccess ? (
            <div role="status" className={page.sentCard}>
              <div className={page.sentIcon}>
                <svg aria-hidden="true" width="26" height="26" viewBox="0 0 24 24">
                  <path d="M3 6 H21 V18 H3 Z M3 6 L12 13 L21 6" className={page.sentIconLine} strokeWidth="2" />
                </svg>
              </div>
              <h1 ref={sentHeading} tabIndex={-1} className={`${page.title} ${page.sentTitle}`}>
                Check your email
              </h1>
              <p className={page.sentText}>
                If that email is on our family list, a reset link is on its way. It works for 30 minutes.
              </p>
              <p className={`${page.sentText} ${page.muted}`}>
                Nothing after a few minutes? Check your spam folder, or ask an admin to send you a new link.
              </p>
              <Link to="/auth/login" className={`pill ${page.secondary} ${styles.backButton}`}>
                Back to sign in
              </Link>
            </div>
          ) : (
            <form className={styles.request} onSubmit={onSubmit}>
              <div className={page.heading}>
                <h1 className={`${page.title} ${styles.title}`}>Forgot your password?</h1>
                <span className={styles.intro}>Enter your email and we’ll send you a link to choose a new password.</span>
              </div>
              <div className={page.field}>
                <label htmlFor="fp-email" className={page.label}>
                  Email
                </label>
                <input
                  id="fp-email"
                  type="email"
                  autoComplete="email"
                  required
                  // Locked while sending, so a corrected typo can't be silently left unsent.
                  readOnly={send.isPending}
                  value={email}
                  onChange={(event) => {
                    setEmail(event.target.value)
                    if (send.isError) {
                      send.reset()
                    }
                  }}
                  className={page.input}
                />
              </div>
              {send.isError && (
                <p role="alert" className={page.error}>
                  {errorMessage(send.error)}
                </p>
              )}
              {/* aria-disabled, not disabled: it keeps keyboard focus while sending. */}
              <button type="submit" aria-disabled={send.isPending} className={`pill ${page.primary}`}>
                {send.isPending ? 'Sending…' : 'Send me a reset link'}
              </button>
            </form>
          )}
        </div>
      </main>
    </>
  )
}
