import { useMutation } from '@tanstack/react-query'
import { type FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router'

import { login } from '../../api/auth'
import { ApiError } from '../../api/client'
import { PasswordInput } from '../../components/PasswordInput/PasswordInput'
import { errorMessage } from '../../lib/errorMessage'
import page from '../../styles/signInForm.module.css'
import styles from './Login.module.css'

export function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const navigate = useNavigate()
  const signIn = useMutation({
    mutationFn: () => login(email, password),
    onSuccess: () => void navigate('/', { replace: true }),
  })

  // Only a shown error: resetting a pending sign-in would detach it, re-enabling the button and losing its answer.
  const clearError = () => {
    if (signIn.isError) {
      signIn.reset()
    }
  }

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    // The button is aria-disabled, not disabled, so it keeps keyboard focus; this does its job.
    if (signIn.isPending) {
      return
    }
    signIn.mutate()
  }

  const error = signIn.error ? loginErrorMessage(signIn.error) : null

  return (
    <>
      <title>Sign in — Pluviose-Monestime Family Hub</title>
      <main className={page.main}>
        <form className={page.form} onSubmit={onSubmit}>
          <div className={page.heading}>
            <h1 lang="ht" className={`${page.title} ${styles.title}`}>
              Lakay se lakay
            </h1>
            <span className={styles.subtitle}>There’s no place like home.</span>
          </div>

          <div className={page.field}>
            <label htmlFor="si-email" className={page.label}>
              Email
            </label>
            <input
              id="si-email"
              type="email"
              autoComplete="email"
              required
              readOnly={signIn.isPending}
              value={email}
              onChange={(event) => {
                setEmail(event.target.value)
                clearError()
              }}
              className={page.input}
            />
          </div>
          <div className={page.field}>
            <div className={styles.labelRow}>
              <label htmlFor="si-pw" className={page.label}>
                Password
              </label>
              <Link to="/auth/forgot" className={styles.small}>
                Forgot password?
              </Link>
            </div>
            <PasswordInput
              id="si-pw"
              autoComplete="current-password"
              readOnly={signIn.isPending}
              value={password}
              onChange={(value) => {
                setPassword(value)
                clearError()
              }}
            />
          </div>

          {error && (
            <p role="alert" className={page.error}>
              {error}
            </p>
          )}

          <button type="submit" aria-disabled={signIn.isPending} className={`pill ${page.primary}`}>
            {signIn.isPending ? 'Signing in…' : 'Sign in'}
          </button>
          <span className={styles.requestAccess}>
            First time here?{' '}
            <Link to="/auth/request-access" className={styles.strong}>
              Request access
            </Link>
          </span>

          <span className={styles.note}>Already invited? Use the link in your invitation email.</span>
        </form>
      </main>
    </>
  )
}

function loginErrorMessage(error: Error): string {
  if (error instanceof ApiError && error.status === 401) {
    return 'That email and password don’t match. Check them and try again, or use Forgot password.'
  }
  return errorMessage(error)
}
