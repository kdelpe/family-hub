import { useMutation, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, Fragment, useState } from 'react'
import { Link } from 'react-router'

import { adminKeys } from '../../api/admin'
import { onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { directoryKeys } from '../../api/contacts'
import { familyKeys } from '../../api/family'
import { type Person, personKeys, updateOwnProfile } from '../../api/persons'
import { treeKeys } from '../../api/tree'
import { bornOn } from '../../lib/dates'
import { refusedMessage } from '../../lib/errorMessage'
import { fullName, initials } from '../../lib/names'
import { useSignedIn } from '../../lib/useSignedIn'
import { ContactDetails } from './ContactDetails'
import styles from './MyProfile.module.css'
import { RequestsCard } from './RequestsCard'
import { SignInCard } from './SignInCard'
import { SuggestCard } from './SuggestCard'

// design-notes §4.8. Photo uploads arrive in Phase 5.
export function MyProfile() {
  const { user, person } = useSignedIn()
  if (!user) {
    return null
  }

  return (
    <main className={styles.page}>
      <div className={styles.column}>
        <h1 className={styles.title}>My profile</h1>
        {person ? (
          // Keyed by person: if an admin relinks the login, nothing typed for one person may be saved to another.
          <Fragment key={person.id}>
            <StoryCard person={person} />
            <ContactDetails personId={person.id} />
          </Fragment>
        ) : user.person_id ? (
          <p role="alert" className={styles.card}>
            Your profile didn’t load. Refresh the page to try again.
          </p>
        ) : (
          <p className={styles.card}>
            Your login isn’t linked to you in the tree yet, so there’s no profile to fill in. An admin can link it.
          </p>
        )}
      </div>
      <aside className={styles.aside}>
        {/* Keyed by person too: answers given and names typed for one person never stay for another. */}
        {person && <RequestsCard key={person.id} personId={person.id} />}
        <SignInCard email={user.email} />
        {person && (
          <>
            <SuggestCard key={person.id} yourName={fullName(person)} />
            <Link to="/tree" className={`pill ${styles.seeMe}`}>
              See me in the tree
            </Link>
          </>
        )}
      </aside>
    </main>
  )
}

function StoryCard({ person }: { person: Person }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<string | null>(null)
  const [justSaved, setJustSaved] = useState(false)
  const saved = (person.bio ?? '').trim()
  const text = draft ?? person.bio ?? ''
  const changed = draft !== null && draft.trim() !== saved
  const save = useMutation({
    mutationFn: () => updateOwnProfile(person.id, { bio: text.trim() || null }),
    onMutate: sessionGeneration,
    onSuccess: (updated, _, since) => {
      if (!sameSession(since)) {
        return
      }
      // Only the story: a details save answering out of order mustn't be undone by this one.
      queryClient.setQueryData<Person>(personKeys.one(person.id), (old) => old && { ...old, bio: updated.bio })
      // A read that left before this save mustn't land after it: fetch what was committed.
      void queryClient.invalidateQueries({ queryKey: personKeys.one(person.id) })
      setDraft(null)
      setJustSaved(true)
    },
    onError: onSessionEnded(queryClient),
  })

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (changed && !save.isPending) {
      save.mutate()
    }
  }

  return (
    <section aria-label="Photo and story" className={styles.card}>
      <Details person={person} />
      <form className={styles.story} onSubmit={onSubmit}>
        <label htmlFor="mp-bio" className={styles.label}>
          Your story
        </label>
        <textarea
          id="mp-bio"
          rows={4}
          // Held still while it saves, so nothing typed meanwhile is lost when the saved text comes back.
          readOnly={save.isPending}
          value={text}
          onChange={(e) => {
            setDraft(e.target.value)
            setJustSaved(false)
            save.reset()
          }}
          className={`${styles.input} ${styles.bio}`}
        />
        <div className={styles.actions}>
          <button
            type="submit"
            aria-disabled={!changed || save.isPending || undefined}
            className={`pill ${styles.save}`}
          >
            Save
          </button>
          {changed && (
            <button
              type="button"
              onClick={() => {
                if (!save.isPending) {
                  setDraft(null)
                  save.reset()
                }
              }}
              aria-disabled={save.isPending || undefined}
              className={`pill ${styles.cancel}`}
            >
              Cancel
            </button>
          )}
        </div>
        <span role="status" className={styles.small}>
          {justSaved && 'Saved.'}
        </span>
        {save.isError && (
          <p role="alert" className={styles.formError}>
            {refusedMessage(save.error)}
          </p>
        )}
      </form>
    </section>
  )
}

// The viewer's own date, not UTC's, which is already tomorrow on a US evening.
const today = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const cameraIcon = (
  <svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24">
    <path d="M4 8h3l2-3h6l2 3h3v11H4z" />
    <circle cx="12" cy="13" r="3.5" />
  </svg>
)

// Name and birthday are each person's to fix: admins can't know every birthday (architecture §4).
function Details({ person }: { person: Person }) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [first, setFirst] = useState('')
  const [last, setLast] = useState('')
  const [date, setDate] = useState('')
  const [noFirst, setNoFirst] = useState(false)
  const [future, setFuture] = useState(false)
  // Only a full date fills the field: a masked one ("--06-06") would show empty, and saving would then clear it.
  const fullDate = /^\d{4}-\d{2}-\d{2}$/.test(person.birth_date ?? '')
  const exactDate = person.birth_date_precision === 'day' && fullDate ? person.birth_date! : ''
  const born = bornOn(person.birth_date, person.birth_date_precision)

  // Only what changed: a partial date on file ("1978") stays unless a full one is picked.
  const lastName = last.trim() || null
  const changes = {
    ...(first.trim() !== person.first_name && { first_name: first.trim() }),
    ...(lastName !== person.last_name && { last_name: lastName }),
    ...(date !== exactDate && date !== '' && { birth_date: date, birth_date_precision: 'day' as const }),
    // Emptying the field clears the date; a partial one shows empty, so it's kept.
    ...(date === '' && exactDate !== '' && { birth_date: null, birth_date_precision: null }),
  }

  const save = useMutation({
    mutationFn: () => updateOwnProfile(person.id, changes),
    onMutate: sessionGeneration,
    onSuccess: (updated, _, since) => {
      if (!sameSession(since)) {
        return
      }
      const { first_name, last_name, birth_date, birth_date_precision } = updated
      queryClient.setQueryData<Person>(
        personKeys.one(person.id),
        (old) => old && { ...old, first_name, last_name, birth_date, birth_date_precision },
      )
      // This profile too (a read that left before the save mustn't land after it), and where else a name or birthday shows.
      for (const queryKey of [
        personKeys.one(person.id),
        personKeys.some([]),
        treeKeys.all,
        directoryKeys.all,
        familyKeys.summary,
        adminKeys.members,
      ]) {
        void queryClient.invalidateQueries({ queryKey })
      }
      setEditing(false)
    },
    onError: onSessionEnded(queryClient),
  })

  const open = () => {
    setFirst(person.first_name)
    setLast(person.last_name ?? '')
    setDate(exactDate)
    setNoFirst(false)
    setFuture(false)
    save.reset()
    setEditing(true)
  }

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (save.isPending) {
      return
    }
    // Checked here: the form is noValidate, so the field's max alone doesn't stop a typed future date.
    const inFuture = date > today()
    setNoFirst(first.trim() === '')
    setFuture(inFuture)
    if (first.trim() === '' || inFuture) {
      return
    }
    if (Object.keys(changes).length === 0) {
      setEditing(false)
      return
    }
    save.mutate()
  }

  if (!editing) {
    return (
      <div className={styles.who}>
        {/* A pointer shortcut to the Edit link, so keyboards and screen readers meet one control, not two. */}
        <button type="button" onClick={open} tabIndex={-1} aria-hidden="true" className={styles.avatarButton}>
          <span className={styles.avatar}>{initials(person)}</span>
          <span className={styles.camera}>{cameraIcon}</span>
        </button>
        <div className={styles.whoWords}>
          <span className={styles.name}>{fullName(person)}</span>
          <span className={styles.muted}>{born ? `Born ${born}` : 'No birthday yet'}</span>
          <button type="button" onClick={open} aria-label="Edit your name, birthday and photo" className={styles.edit}>
            Edit
          </button>
        </div>
      </div>
    )
  }

  return (
    <form noValidate onSubmit={onSubmit} aria-label="Your details" className={styles.who}>
      <div className={styles.photo}>
        <span aria-hidden="true" className={styles.avatar}>
          {initials(person)}
        </span>
        <span className={styles.small}>Photos are coming soon.</span>
      </div>
      <div className={styles.detailsFields}>
        <div className={styles.nameFields}>
          <div className={styles.field}>
            <label htmlFor="mp-first" className={styles.label}>
              First name
            </label>
            <input
              id="mp-first"
              autoComplete="given-name"
              readOnly={save.isPending}
              value={first}
              onChange={(e) => setFirst(e.target.value)}
              aria-invalid={noFirst || undefined}
              aria-describedby={noFirst ? 'mp-first-error' : undefined}
              className={styles.input}
            />
            {noFirst && (
              <span id="mp-first-error" role="alert" className={styles.fieldError}>
                Enter your first name.
              </span>
            )}
          </div>
          <div className={styles.field}>
            <label htmlFor="mp-last" className={styles.label}>
              Last name
            </label>
            <input
              id="mp-last"
              autoComplete="family-name"
              readOnly={save.isPending}
              value={last}
              onChange={(e) => setLast(e.target.value)}
              className={styles.input}
            />
          </div>
        </div>
        <div className={styles.field}>
          <label htmlFor="mp-birthday" className={styles.label}>
            Birthday
          </label>
          <input
            id="mp-birthday"
            type="date"
            readOnly={save.isPending}
            value={date}
            max={today()}
            aria-invalid={future || undefined}
            aria-describedby={future ? 'mp-birthday-error' : born && !exactDate ? 'mp-birthday-hint' : undefined}
            onChange={(e) => setDate(e.target.value)}
            className={`${styles.input} ${styles.date}`}
          />
          {future && (
            <span id="mp-birthday-error" role="alert" className={styles.fieldError}>
              Pick a date that isn’t in the future.
            </span>
          )}
          {born && !exactDate && (
            <span id="mp-birthday-hint" className={styles.small}>
              On file: {born}. Pick the full date to change it.
            </span>
          )}
        </div>
        {save.isError && (
          <p role="alert" className={styles.formError}>
            {refusedMessage(save.error)}
          </p>
        )}
        <div className={styles.actions}>
          <button type="submit" aria-disabled={save.isPending || undefined} className={`pill ${styles.save}`}>
            Save
          </button>
          <button
            type="button"
            onClick={() => !save.isPending && setEditing(false)}
            aria-disabled={save.isPending || undefined}
            className={`pill ${styles.cancel}`}
          >
            Cancel
          </button>
        </div>
      </div>
    </form>
  )
}
