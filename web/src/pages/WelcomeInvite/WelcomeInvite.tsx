import { useMutation, useQuery } from '@tanstack/react-query'
import { type FormEvent, useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'

import { authKeys, googleStartPath, type LinkPreview, previewLink, providersQuery, setPassword } from '../../api/auth'
import { ApiError } from '../../api/client'
import { PasswordInput } from '../../components/PasswordInput/PasswordInput'
import { PasswordStrength } from '../../components/PasswordStrength/PasswordStrength'
import { errorMessage } from '../../lib/errorMessage'
import { maxPasswordLength, minPasswordLength, passwordLength } from '../../lib/passwordStrength'
import page from '../../styles/signInForm.module.css'
import styles from './WelcomeInvite.module.css'

const passwordErrorId = 'wi-pw-error'

// The token rides in the URL fragment (#t=…), so it never reaches server logs or a Referer header.
function tokenFrom(hash: string): string {
  return new URLSearchParams(hash.slice(1)).get('t') ?? ''
}

export function WelcomeInvite() {
  const { hash, pathname, search } = useLocation()
  const navigate = useNavigate()
  const [token, setToken] = useState(() => tokenFrom(hash))
  const linkInAddress = tokenFrom(hash)
  // A new link pasted into this tab changes only the fragment, so the page stays mounted.
  if (linkInAddress !== '' && linkInAddress !== token) {
    setToken(linkInAddress)
  }
  // Off the address bar and this tab's Back button. The browser's history list has already recorded it.
  useEffect(() => {
    if (hash !== '') {
      void navigate({ pathname, search }, { replace: true })
    }
  }, [hash, pathname, search, navigate])
  const preview = useQuery({
    queryKey: authKeys.preview(token),
    queryFn: ({ signal }) => previewLink(token, { signal }),
    enabled: token !== '',
    staleTime: Infinity,
    // An errored query counts as stale; without these a dead link is re-sent to a rate-limited endpoint on every tab switch.
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
  // Fetched alongside the preview, so the Google button can be in place before the form shows.
  const providers = useQuery(providersQuery)

  const linkBroken = token === '' || (preview.error instanceof ApiError && preview.error.code === 'invalid_token')
  const mayOfferGoogle = preview.data?.purpose === 'invite' && preview.data.google_eligible

  return (
    <main className={page.main}>
      {linkBroken ? (
        <BrokenLink tokenMissing={token === ''} />
      ) : preview.isError ? (
        <div className={`${page.form} ${styles.column}`}>
          <PageTitle>Welcome</PageTitle>
          <p role="alert" className={page.error}>
            {errorMessage(preview.error)}
          </p>
          <button
            type="button"
            aria-disabled={preview.isFetching}
            onClick={() => {
              if (!preview.isFetching) {
                void preview.refetch()
              }
            }}
            className={`pill ${page.primary}`}
          >
            {preview.isFetching ? 'Trying…' : 'Try again'}
          </button>
        </div>
      ) : preview.data && !(mayOfferGoogle && providers.isPending) ? (
        // As on the sign-in page, a failed check still offers Google: the link either works or explains.
        <SetPassword key={token} token={token} link={preview.data} googleAvailable={providers.data?.google !== false} />
      ) : (
        <PageTitle>Welcome</PageTitle>
      )}
    </main>
  )
}

// Each view names the tab itself, so the title follows when a submit turns the form into BrokenLink.
function PageTitle({ children }: { children: string }) {
  return <title>{`${children} — Pluviose-Monestime Family Hub`}</title>
}

function SetPassword({ token, link, googleAvailable }: { token: string; link: LinkPreview; googleAvailable: boolean }) {
  const invite = link.purpose === 'invite'
  const [password, setPasswordValue] = useState('')
  const [refusedHere, setRefusedHere] = useState<{ code: string; attempt: number } | null>(null)
  const navigate = useNavigate()
  const save = useMutation({
    mutationFn: () => setPassword(token, password),
    onSuccess: () => void navigate(invite ? '/setup' : '/', { replace: true }),
  })

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (save.isPending) {
      return
    }
    const length = passwordLength(password)
    if (length < minPasswordLength || length > maxPasswordLength) {
      setRefusedHere({ code: length < minPasswordLength ? 'password_too_short' : 'password_too_long', attempt: (refusedHere?.attempt ?? 0) + 1 })
      return
    }
    save.mutate()
  }

  const errorCode = save.error instanceof ApiError ? save.error.code : null
  if (errorCode === 'invalid_token') {
    return <BrokenLink focusHeading />
  }
  const fieldError = passwordRule(refusedHere?.code ?? errorCode)
  const otherError = save.isError && !fieldError ? errorMessage(save.error) : null
  const firstName = link.invitee_first_name
  const withGoogle = invite && link.google_eligible && googleAvailable

  return (
    // noValidate: an empty field gets the plain-language error under it, not the browser's bubble.
    <form noValidate className={`${page.form} ${styles.column}`} onSubmit={onSubmit}>
      <PageTitle>{invite ? 'Welcome' : 'Choose a new password'}</PageTitle>
      {/* For password managers, so the saved password is filed under this email. */}
      <input type="email" name="username" autoComplete="username" value={link.invitee_email} readOnly hidden />
      {invite && link.from_name && (
        <div className={styles.from}>
          <span aria-hidden="true" className={styles.avatar}>
            {initials(link.from_name)}
          </span>
          <span className={styles.fromText}>
            <strong>{link.from_name}</strong> invited you to the family hub.
          </span>
        </div>
      )}
      <div className={page.heading}>
        <h1 className={`${page.title} ${styles.title}`}>
          {invite ? (firstName ? `Welcome to the family, ${firstName}` : 'Welcome to the family') : 'Choose a new password'}
        </h1>
        <span className={styles.intro}>
          {invite ? 'Choose a password to finish setting up your account for ' : 'Choose a new password for '}
          <strong className={styles.email}>{link.invitee_email}</strong>.
        </span>
      </div>

      {withGoogle && (
        <>
          <a href={googleStartPath} className={`pill ${page.secondary} ${page.googleButton}`}>
            Continue with Google instead
          </a>
          <div aria-hidden="true" className={page.divider}>
            <span className={page.rule} />
            or choose a password
            <span className={page.rule} />
          </div>
        </>
      )}

      <div className={page.field}>
        <label htmlFor="wi-pw" className={page.label}>
          New password
        </label>
        <PasswordInput
          id="wi-pw"
          autoComplete="new-password"
          initiallyShown
          readOnly={save.isPending}
          value={password}
          onChange={(value) => {
            setPasswordValue(value)
            setRefusedHere(null)
            if (save.isError) {
              save.reset()
            }
          }}
          describedBy={fieldError ? passwordErrorId : undefined}
          invalid={fieldError !== null}
        />
        <PasswordStrength password={password} />
        {fieldError && (
          // Keyed by attempt: a new node is announced again when the same refusal repeats.
          <span key={refusedHere?.attempt} id={passwordErrorId} role="alert" className={styles.fieldError}>
            {fieldError}
          </span>
        )}
      </div>

      {otherError && (
        <p role="alert" className={page.error}>
          {otherError}
        </p>
      )}

      {/* aria-disabled, not disabled: it keeps keyboard focus while saving. */}
      <button type="submit" aria-disabled={save.isPending} className={`pill ${page.primary}`}>
        {save.isPending ? 'Saving…' : invite ? 'Set password and continue' : 'Set password and sign in'}
      </button>

      <span className={styles.note}>
        This link works for {invite ? '7 days' : '30 minutes'}. Not {invite && firstName ? firstName : 'you'}?{' '}
        <Link to="/auth/login">Go to sign in</Link>
      </span>
    </form>
  )
}

// focusHeading: when it replaces the form after a submit, so focus doesn't fall back to the page.
// tokenMissing: the page was reloaded (the token is dropped from the URL once read) or opened bare.
function BrokenLink({ focusHeading = false, tokenMissing = false }: { focusHeading?: boolean; tokenMissing?: boolean }) {
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    if (focusHeading) {
      heading.current?.focus()
    }
  }, [focusHeading])
  return (
    <div className={`${page.form} ${styles.column}`}>
      <PageTitle>This link doesn’t work anymore</PageTitle>
      <div className={page.heading}>
        <h1 ref={heading} tabIndex={-1} className={`${page.title} ${styles.title}`}>
          This link doesn’t work anymore
        </h1>
        <span className={styles.intro}>
          {tokenMissing ? 'Open it again from your email.' : 'It may have expired or already been used.'}
        </span>
      </div>
      <p className={styles.text}>
        Invitation: ask an admin for a new one.
        <br />
        Password reset: request a new link.
      </p>
      <Link to="/auth/forgot" className={`pill ${page.primary}`}>
        Get a new reset link
      </Link>
      <span className={styles.note}>
        <Link to="/auth/login">Go to sign in</Link>
      </span>
    </div>
  )
}

// The plain-language rule a refused password broke (design-notes §3.3), or null for any other failure.
function passwordRule(code: string | null): string | null {
  switch (code) {
    case 'password_too_short':
      return `Use at least ${minPasswordLength} characters.`
    case 'password_too_long':
      return `Use at most ${maxPasswordLength} characters.`
    case 'password_too_common':
      return 'That password is too common — try a short phrase.'
    default:
      return null
  }
}

function initials(name: string): string {
  const words = name.normalize('NFC').trim().split(/\s+/)
  // By code point, so a name starting with an astral character isn't cut in half.
  const initial = (word: string | undefined) => (word ? ([...word][0] ?? '') : '')
  const first = initial(words[0])
  const last = words.length > 1 ? initial(words[words.length - 1]) : ''
  return (first + last).toUpperCase()
}
