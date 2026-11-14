import { useMutation, useQuery } from '@tanstack/react-query'
import { type FormEvent, useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'

import { authKeys, googleStartPath, login, providers } from '../../api/auth'
import { ApiError, networkErrorStatus } from '../../api/client'
import styles from './Login.module.css'

// Not "try again": the account may be refused for good (not invited), which a retry can't fix.
const googleFailedMessage =
  'Signing in with Google didn’t work. Sign in with your email and password, or request access if you’re new here.'
const googleOffMessage = 'Signing in with Google isn’t available. Sign in with your email and password.'

export function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordShown, setPasswordShown] = useState(false)
  const [searchParams, setSearchParams] = useSearchParams()
  // Read once, then dropped from the address, so a reload doesn't show it again.
  const [googleFailed, setGoogleFailed] = useState(searchParams.get('google') === 'unavailable')
  const navigate = useNavigate()

  useEffect(() => {
    if (searchParams.has('google')) {
      const rest = new URLSearchParams(searchParams)
      rest.delete('google')
      setSearchParams(rest, { replace: true })
    }
  }, [searchParams, setSearchParams])

  const { data: available, isPending: providersPending } = useQuery({
    queryKey: authKeys.providers,
    queryFn: providers,
    staleTime: Infinity,
    // One quick check: retrying would hold back the Google button and message for seconds.
    retry: false,
  })
  const signIn = useMutation({
    mutationFn: () => login(email, password),
    onSuccess: () => void navigate('/', { replace: true }),
  })

  // Only a shown error: resetting a pending sign-in would detach it, re-enabling the button and losing its answer.
  const clearError = () => {
    setGoogleFailed(false)
    if (signIn.isError) {
      signIn.reset()
    }
  }

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setGoogleFailed(false)
    signIn.mutate()
  }

  const error = signIn.error
    ? loginErrorMessage(signIn.error)
    : // Waits for providers, so the alert isn't announced once and then corrected.
      googleFailed && !providersPending
      ? available?.google === false
        ? googleOffMessage
        : googleFailedMessage
      : null

  return (
    <>
      <title>Sign in — Pluviose-Monestime Family Hub</title>
      <main className={styles.main}>
        <form className={styles.form} onSubmit={onSubmit}>
          <div className={styles.heading}>
            <h1 lang="ht" className={styles.title}>
              Lakay se lakay
            </h1>
            <span className={styles.subtitle}>There’s no place like home.</span>
          </div>

          <div className={styles.field}>
            <label htmlFor="si-email" className={styles.label}>
              Email
            </label>
            <input
              id="si-email"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(event) => {
                setEmail(event.target.value)
                clearError()
              }}
              className={styles.input}
            />
          </div>
          <div className={styles.field}>
            <div className={styles.labelRow}>
              <label htmlFor="si-pw" className={styles.label}>
                Password
              </label>
              <Link to="/auth/forgot" className={styles.small}>
                Forgot password?
              </Link>
            </div>
            <div className={styles.passwordBox}>
              <input
                id="si-pw"
                type={passwordShown ? 'text' : 'password'}
                autoComplete="current-password"
                required
                value={password}
                onChange={(event) => {
                  setPassword(event.target.value)
                  clearError()
                }}
                className={styles.passwordInput}
              />
              <button
                type="button"
                aria-pressed={passwordShown}
                onClick={() => setPasswordShown(!passwordShown)}
                className={styles.reveal}
              >
                {passwordShown ? 'Hide' : 'Show'}
              </button>
            </div>
          </div>

          {error && (
            <p role="alert" className={styles.error}>
              {error}
            </p>
          )}

          <button type="submit" disabled={signIn.isPending} className={`pill ${styles.primary}`}>
            {signIn.isPending ? 'Signing in…' : 'Sign in'}
          </button>
          <span className={styles.requestAccess}>
            First time here?{' '}
            <Link to="/auth/request-access" className={styles.strong}>
              Request access
            </Link>
          </span>

          {/* Holds Google's place until the answer comes, so the centered form doesn't jump. */}
          {/* Shown too if the check failed: the link then either works or comes back here with a message. */}
          {available?.google !== false && (
            <div className={providersPending ? `${styles.google} ${styles.reserved}` : styles.google}>
              <div aria-hidden="true" className={styles.divider}>
                <span className={styles.rule} />
                or
                <span className={styles.rule} />
              </div>
              <a href={googleStartPath} className={`pill ${styles.secondary}`}>
                Continue with Google
              </a>
            </div>
          )}

          <span className={styles.note}>Already invited? Use the link in your invitation email.</span>
        </form>
      </main>
    </>
  )
}

function loginErrorMessage(error: Error): string {
  if (!(error instanceof ApiError)) {
    return 'Something went wrong. Try again in a moment.'
  }
  switch (error.status) {
    case 401:
      return 'That email and password don’t match. Check them and try again, or use Forgot password.'
    case 429: {
      const minutes = Math.max(1, Math.ceil((error.retryAfterSeconds ?? 60) / 60))
      return `Too many tries. Wait ${minutes} ${minutes === 1 ? 'minute' : 'minutes'} and try again.`
    }
    case 503:
      return 'The family hub is busy right now. Try again in a moment.'
    case networkErrorStatus:
      return 'Can’t reach the family hub. Check your connection and try again.'
    default:
      return 'Something went wrong. Try again in a moment.'
  }
}
