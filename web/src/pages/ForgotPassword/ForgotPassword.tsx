import { useMutation } from '@tanstack/react-query'
import { type FormEvent, useState } from 'react'
import { Link } from 'react-router'

import { forgotPassword } from '../../api/auth'
import { errorMessage } from '../../api/errorMessage'
import page from '../../components/SignInLayout/SignInForm.module.css'
import styles from './ForgotPassword.module.css'

export function ForgotPassword() {
  const [email, setEmail] = useState('')
  const send = useMutation({ mutationFn: () => forgotPassword(email) })

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    send.mutate()
  }

  return (
    <>
      <title>Forgot password — Pluviose-Monestime Family Hub</title>
      <main className={page.main}>
        <div className={page.form}>
          <Link to="/auth/login" className={styles.back}>
            ← Back to sign in
          </Link>
          {send.isSuccess ? (
            <div role="status" className={styles.sent}>
              <div className={styles.envelope}>
                <svg aria-hidden="true" width="26" height="26" viewBox="0 0 24 24">
                  <path d="M3 6 H21 V18 H3 Z M3 6 L12 13 L21 6" className={styles.envelopeLine} strokeWidth="2" />
                </svg>
              </div>
              <h1 className={styles.sentTitle}>Check your email</h1>
              <p className={styles.sentText}>
                If that email is on our family list, a reset link is on its way. It works for 30 minutes.
              </p>
              <p className={`${styles.sentText} ${styles.muted}`}>
                Nothing after a few minutes? Check your spam folder, or ask an admin to send you a new link.
              </p>
              <Link to="/auth/login" className={`pill ${styles.backButton}`}>
                Back to sign in
              </Link>
            </div>
          ) : (
            <form className={styles.request} onSubmit={onSubmit}>
              <div className={styles.heading}>
                <h1 className={styles.title}>Forgot your password?</h1>
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
              <button type="submit" disabled={send.isPending} className={`pill ${page.primary}`}>
                {send.isPending ? 'Sending…' : 'Send me a reset link'}
              </button>
              <span className={styles.hint}>
                Signed up with Google? You can still use “Continue with Google” on the sign-in page, or use this to add a
                password.
              </span>
            </form>
          )}
        </div>
      </main>
    </>
  )
}
