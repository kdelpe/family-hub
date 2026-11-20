import { useMutation } from '@tanstack/react-query'
import { type FormEvent, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'

import { type FamilySide, relationships, requestAccess } from '../../api/accessRequests'
import { errorMessage } from '../../lib/errorMessage'
import { familySides } from '../../lib/familySides'
import page from '../../styles/signInForm.module.css'
import styles from './RequestAccess.module.css'

const maxNames = 4

export function RequestAccess() {
  const [fullName, setFullName] = useState('')
  const [email, setEmail] = useState('')
  const [side, setSide] = useState<FamilySide | null>(null)
  const [relationship, setRelationship] = useState('daughter')
  const [names, setNames] = useState(['', ''])
  const [note, setNote] = useState('')
  const [website, setWebsite] = useState('')
  const [nameMissing, setNameMissing] = useState(false)
  const [addedName, setAddedName] = useState<number | null>(null)
  const nameInput = useRef<HTMLInputElement>(null)
  const send = useMutation({
    mutationFn: () =>
      requestAccess({
        full_name: fullName.trim(),
        email: email.trim(),
        side,
        relationship,
        related_names: names.map((name) => name.trim()).filter((name) => name !== ''),
        note: note.trim(),
        website,
      }),
  })
  const sentHeading = useRef<HTMLHeadingElement>(null)

  // The focused button goes away with the form; the heading takes focus, so keyboard users land on the result.
  useEffect(() => {
    if (send.isSuccess) {
      sentHeading.current?.focus()
    }
  }, [send.isSuccess])

  // The add button disappears at the last field, so focus goes to the field it added.
  useEffect(() => {
    if (addedName !== null) {
      document.getElementById(`ra-of-${addedName}`)?.focus()
    }
  }, [addedName])

  const clearError = () => {
    setNameMissing(false)
    if (send.isError) {
      send.reset()
    }
  }

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (send.isPending) {
      return
    }
    // `required` lets a name of only spaces through, and the server refuses a blank one.
    if (fullName.trim() === '') {
      setNameMissing(true)
      nameInput.current?.focus()
      return
    }
    send.mutate()
  }

  const firstName = fullName.trim().split(/\s+/)[0]

  return (
    <>
      <title>Request access — Pluviose-Monestime Family Hub</title>
      <main className={`${page.main} ${page.tallMain}`}>
        <div className={`${page.form} ${styles.column}`}>
          <Link to="/auth/login" className={page.backLink}>
            <span aria-hidden="true">← </span>Back to sign in
          </Link>
          {send.isSuccess ? (
            <div role="status" className={page.sentCard}>
              <div className={page.sentIcon}>
                <svg aria-hidden="true" width="26" height="26" viewBox="0 0 24 24">
                  <path d="M5 12.5 L10 17 L19 7" className={page.sentIconLine} strokeWidth="2.4" strokeLinecap="round" />
                </svg>
              </div>
              <h1 ref={sentHeading} tabIndex={-1} className={`${page.title} ${page.sentTitle}`}>
                {firstName ? `Thank you, ${firstName}` : 'Thank you'}
              </h1>
              <p className={page.sentText}>
                Your request is with the admins. If they approve it, you’ll get an invitation email with a link to set up
                your account — usually within a few days.
              </p>
              <p className={`${page.sentText} ${page.muted}`}>Know one of the admins? A quick call or text can speed things up.</p>
            </div>
          ) : (
            <form aria-labelledby="ra-title" className={styles.request} onSubmit={onSubmit}>
              <div className={`${page.heading} ${styles.heading}`}>
                <h1 id="ra-title" className={`${page.title} ${styles.title}`}>
                  Request access
                </h1>
                <p className={styles.intro}>
                  Tell us a little about yourself and an admin will take a look. If they approve, you’ll get an invitation
                  by email.
                </p>
              </div>

              <div className={page.field}>
                <label htmlFor="ra-name" className={page.label}>
                  Your full name
                </label>
                <input
                  ref={nameInput}
                  id="ra-name"
                  type="text"
                  autoComplete="name"
                  required
                  readOnly={send.isPending}
                  value={fullName}
                  onChange={(event) => {
                    setFullName(event.target.value)
                    clearError()
                  }}
                  className={page.input}
                />
              </div>
              <div className={page.field}>
                <label htmlFor="ra-email" className={page.label}>
                  Email
                </label>
                <input
                  id="ra-email"
                  type="email"
                  autoComplete="email"
                  required
                  readOnly={send.isPending}
                  value={email}
                  onChange={(event) => {
                    setEmail(event.target.value)
                    clearError()
                  }}
                  className={page.input}
                />
              </div>

              <fieldset className={styles.group}>
                <legend className={`${page.label} ${styles.sideLegend}`}>How are you connected to the family?</legend>
                <div className={styles.sides}>
                  {familySides.map((option) => (
                    <label key={option.value} className={styles.side}>
                      <input
                        type="radio"
                        name="side"
                        value={option.value}
                        checked={side === option.value}
                        onChange={() => {
                          if (!send.isPending) {
                            setSide(option.value)
                            clearError()
                          }
                        }}
                        className="visually-hidden"
                      />
                      {option.label}
                    </label>
                  ))}
                </div>
              </fieldset>

              <fieldset className={styles.group}>
                <legend className={`${page.label} ${styles.relatedLegend}`}>How are you related?</legend>
                <span className={page.hint}>
                  Name a parent, grandparent or sibling if you can — it helps us find your place in the tree.
                </span>
                <div className={styles.sentence}>
                  <span className={styles.words}>I’m the</span>
                  <label htmlFor="ra-rel" className="visually-hidden">
                    Relationship
                  </label>
                  <select
                    id="ra-rel"
                    value={relationship}
                    aria-disabled={send.isPending}
                    onChange={(event) => {
                      if (!send.isPending) {
                        setRelationship(event.target.value)
                        clearError()
                      }
                    }}
                    className={styles.select}
                  >
                    {relationships.map((option) => (
                      <option key={option}>{option}</option>
                    ))}
                  </select>
                  <span className={styles.words}>of</span>
                  {names.map((name, index) => (
                    // Index keys: names are only ever added at the end, never removed or reordered.
                    <span key={index} className={styles.name}>
                      {index > 0 && <span className={styles.words}>and</span>}
                      <label htmlFor={`ra-of-${index}`} className="visually-hidden">
                        {index === 0 ? 'Their full name' : 'Another full name'}
                      </label>
                      <input
                        id={`ra-of-${index}`}
                        type="text"
                        placeholder="Their full name"
                        readOnly={send.isPending}
                        value={name}
                        onChange={(event) => {
                          setNames(names.map((other, i) => (i === index ? event.target.value : other)))
                          clearError()
                        }}
                        className={styles.nameInput}
                      />
                    </span>
                  ))}
                </div>
                {names.length < maxNames && (
                  <button
                    type="button"
                    aria-disabled={send.isPending}
                    onClick={() => {
                      if (!send.isPending) {
                        setNames([...names, ''])
                        setAddedName(names.length)
                      }
                    }}
                    className={styles.addName}
                  >
                    + Add another name
                  </button>
                )}
              </fieldset>

              <div className={page.field}>
                <label htmlFor="ra-note" className={page.label}>
                  Anything else? <span className={styles.optional}>(optional)</span>
                </label>
                <textarea
                  id="ra-note"
                  rows={2}
                  readOnly={send.isPending}
                  value={note}
                  onChange={(event) => {
                    setNote(event.target.value)
                    clearError()
                  }}
                  className={styles.note}
                />
              </div>

              <div aria-hidden="true" className={styles.trap}>
                {/* Named so no autofill recognizes it: a real person's homepage filled in here would drop their request. */}
                <label htmlFor="ra-website">Leave this empty</label>
                <input
                  id="ra-website"
                  name="ra-extra"
                  type="text"
                  tabIndex={-1}
                  autoComplete="off"
                  value={website}
                  onChange={(event) => setWebsite(event.target.value)}
                />
              </div>

              {nameMissing ? (
                <p role="alert" className={page.error}>
                  Enter your full name.
                </p>
              ) : (
                send.isError && (
                  <p role="alert" className={page.error}>
                    {errorMessage(send.error)}
                  </p>
                )
              )}
              {/* aria-disabled, not disabled: it keeps keyboard focus while sending. */}
              <button type="submit" aria-disabled={send.isPending} className={`pill ${page.primary}`}>
                {send.isPending ? 'Sending…' : 'Send my request'}
              </button>
              <span className={page.hint}>We only use this to check that you’re family. It’s seen by the admins and nobody else.</span>
            </form>
          )}
        </div>
      </main>
    </>
  )
}
