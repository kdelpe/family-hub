import { useIsMutating, useMutation, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useState } from 'react'
import { useNavigate } from 'react-router'

import { changePassword, endSession, logout, onSessionEnded, sessionGeneration } from '../../api/auth'
import { ApiError } from '../../api/client'
import { PasswordInput } from '../../components/PasswordInput/PasswordInput'
import { PasswordStrength } from '../../components/PasswordStrength/PasswordStrength'
import { errorMessage } from '../../lib/errorMessage'
import { lengthRefusal, passwordRule } from '../../lib/passwordStrength'
import styles from './MyProfile.module.css'

const signInKey = ['sign-in'] as const

export function SignInCard({ email }: { email: string }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [changing, setChanging] = useState(false)
  const [changed, setChanged] = useState(false)

  // One at a time: a password change answering after sign-out would set a fresh cookie and sign them back in.
  const signInBusy = useIsMutating({ mutationKey: signInKey }) > 0
  const signOut = useMutation({
    mutationKey: signInKey,
    mutationFn: logout,
    // Off this page first, so the shell isn't left to refetch with the ended session.
    onSuccess: async () => {
      await navigate('/auth/login', { replace: true })
      endSession(queryClient)
    },
    // The server clears the cookie before ending the session, so its own failure still signed them out. A
    // proxy's or the cross-origin check's refusal never reached it.
    onError: async (error) => {
      if (error instanceof ApiError && error.status === 500 && error.code === 'internal') {
        await navigate('/auth/login', { replace: true })
        endSession(queryClient)
      }
    },
  })

  return (
    <section aria-labelledby="mp-signin" className={styles.side}>
      <h2 id="mp-signin" className={styles.sideTitle}>
        Sign-in
      </h2>
      <span className={styles.muted}>Password set</span>
      {changing ? (
        <PasswordForm
          email={email}
          onDone={(done) => {
            setChanging(false)
            setChanged(done)
          }}
        />
      ) : (
        <button
          type="button"
          onClick={() => {
            setChanged(false)
            setChanging(true)
          }}
          className={`pill ${styles.sideButton}`}
        >
          Change password
        </button>
      )}
      <span role="status" className={styles.small}>
        {changed && 'Password changed. You’re signed out on your other devices.'}
      </span>
      <button
        type="button"
        onClick={() => !signInBusy && signOut.mutate()}
        aria-disabled={signInBusy || undefined}
        className={styles.signOut}
      >
        Sign out
      </button>
      {signOut.isError && <p role="alert">{errorMessage(signOut.error)}</p>}
    </section>
  )
}

function PasswordForm({ email, onDone }: { email: string; onDone: (changed: boolean) => void }) {
  const queryClient = useQueryClient()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [refusedHere, setRefusedHere] = useState<string | null>(null)
  // Checked here: an empty try would still use up one of the few the server allows.
  const [noCurrent, setNoCurrent] = useState(false)
  // Remounts the error each try, so the same refusal is announced again.
  const [attempt, setAttempt] = useState(0)
  const signInBusy = useIsMutating({ mutationKey: signInKey }) > 0
  const save = useMutation({
    mutationKey: signInKey,
    mutationFn: () => changePassword(current, next),
    onMutate: sessionGeneration,
    onSuccess: () => onDone(true),
    onError: onSessionEnded(queryClient),
  })

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (signInBusy) {
      return
    }
    setAttempt((n) => n + 1)
    const refused = lengthRefusal(next)
    setNoCurrent(current === '')
    setRefusedHere(refused)
    if (current === '' || refused) {
      save.reset()
      return
    }
    save.mutate()
  }

  const code = save.error instanceof ApiError ? save.error.code : null
  const wrongCurrent = code === 'wrong_password'
  const currentError = noCurrent ? 'Enter your current password.' : wrongCurrent ? 'That’s not your current password.' : null
  const newRule = passwordRule(refusedHere ?? code)
  const other = save.isError && !wrongCurrent && !newRule ? errorMessage(save.error) : null

  return (
    <form noValidate className={styles.passwordForm} onSubmit={onSubmit} aria-label="Change password">
      {/* For password managers, so the new password is filed under this email. */}
      <input type="email" name="username" autoComplete="username" value={email} readOnly hidden />
      <div className={styles.field}>
        <label htmlFor="mp-current" className={styles.label}>
          Current password
        </label>
        <PasswordInput
          id="mp-current"
          autoComplete="current-password"
          value={current}
          readOnly={save.isPending}
          onChange={(value) => {
            setCurrent(value)
            if (wrongCurrent) {
              save.reset()
            }
          }}
          invalid={currentError !== null}
          describedBy={currentError ? 'mp-current-error' : undefined}
        />
        {currentError && (
          <span key={attempt} id="mp-current-error" role="alert" className={styles.fieldError}>
            {currentError}
          </span>
        )}
      </div>
      <div className={styles.field}>
        <label htmlFor="mp-new" className={styles.label}>
          New password
        </label>
        <PasswordInput
          id="mp-new"
          autoComplete="new-password"
          value={next}
          readOnly={save.isPending}
          onChange={setNext}
          invalid={newRule !== null}
          describedBy={newRule ? 'mp-new-error' : undefined}
        />
        <PasswordStrength password={next} />
        {newRule && (
          <span key={attempt} id="mp-new-error" role="alert" className={styles.fieldError}>
            {newRule}
          </span>
        )}
      </div>
      {other && (
        <p role="alert" className={styles.formError}>
          {other}
        </p>
      )}
      <div className={styles.actions}>
        <button type="submit" aria-disabled={save.isPending || undefined} className={`pill ${styles.save}`}>
          Save
        </button>
        <button
          type="button"
          onClick={() => !save.isPending && onDone(false)}
          aria-disabled={save.isPending || undefined}
          className={`pill ${styles.cancel}`}
        >
          Cancel
        </button>
      </div>
    </form>
  )
}
