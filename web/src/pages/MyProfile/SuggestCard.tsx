import { useMutation, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useEffect, useId, useRef, useState } from 'react'

import { relationships, suggestRelative } from '../../api/accessRequests'
import { authKeys, onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { ApiError } from '../../api/client'
import { refusedMessage } from '../../lib/errorMessage'
import styles from './MyProfile.module.css'

// design-notes §4.8. The mockup draws only the button; the form was settled with the owner.
export function SuggestCard({ personId, yourName }: { personId: string; yourName: string }) {
  const [open, setOpen] = useState(false)
  // The first name of whoever was just suggested, for the thank-you line.
  const [sent, setSent] = useState<string | null>(null)
  // Where keyboard focus goes once the form closes: back to the button, or to the thank-you line.
  const [focusAfter, setFocusAfter] = useState<'button' | 'thanks' | null>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const thanksRef = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    if (focusAfter) {
      ;(focusAfter === 'thanks' ? thanksRef : buttonRef).current?.focus()
    }
  }, [focusAfter])

  return (
    <section aria-labelledby="mp-suggest" className={styles.side}>
      <h2 id="mp-suggest" className={styles.sideTitle}>
        Know someone who should be here?
      </h2>
      <span ref={thanksRef} tabIndex={-1} role="status" className={styles.sideText}>
        {sent && `Thanks. An admin will take a look, and if they approve, ${sent} gets an invitation by email.`}
      </span>
      {open ? (
        <SuggestForm
          personId={personId}
          yourName={yourName}
          onDone={(sentTo) => {
            setOpen(false)
            setSent(sentTo)
            setFocusAfter(sentTo ? 'thanks' : 'button')
          }}
        />
      ) : (
        <>
          {!sent && (
            <p className={styles.sideText}>
              Suggest a relative and an admin will take a look. If they approve, your relative gets an invitation by email.
            </p>
          )}
          <button
            ref={buttonRef}
            type="button"
            onClick={() => {
              setSent(null)
              setFocusAfter(null)
              setOpen(true)
            }}
            className={`pill ${styles.suggest}`}
          >
            {sent ? 'Suggest someone else' : 'Suggest a relative'}
          </button>
        </>
      )}
    </section>
  )
}

function SuggestForm({
  personId,
  yourName,
  onDone,
}: {
  personId: string
  yourName: string
  onDone: (sentTo: string | null) => void
}) {
  const id = useId()
  const nameRef = useRef<HTMLInputElement>(null)
  // Straight to the first field, so the keyboard isn't left on the button the form replaced.
  useEffect(() => nameRef.current?.focus(), [])
  const queryClient = useQueryClient()
  const [fullName, setFullName] = useState('')
  const [email, setEmail] = useState('')
  const [relationship, setRelationship] = useState('')
  // Most people suggest their own relatives; the admins' queue then finds this name in the tree.
  const [relatedTo, setRelatedTo] = useState(yourName)
  const [note, setNote] = useState('')
  const [noName, setNoName] = useState(false)
  const send = useMutation({
    mutationFn: () =>
      suggestRelative({
        full_name: fullName.trim(),
        email: email.trim(),
        relationship: relationship || null,
        related_names: relatedTo.trim() ? [relatedTo.trim()] : [],
        note: note.trim(),
        person_id: personId,
      }),
    onMutate: sessionGeneration,
    onSuccess: (_, __, since) => {
      if (sameSession(since)) {
        onDone(fullName.trim().split(/\s+/)[0] ?? fullName.trim())
      }
    },
    onError: (error, variables, since) => {
      onSessionEnded(queryClient)(error, variables, since)
      // Relinked: who you are is read again, and the page moves on to them.
      if (error instanceof ApiError && error.status === 409) {
        void queryClient.refetchQueries({ queryKey: authKeys.me })
      }
    },
  })

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (send.isPending) {
      return
    }
    // The server refuses a blank name too, but in its own words.
    const blank = fullName.trim() === ''
    setNoName(blank)
    if (blank) {
      send.reset()
      return
    }
    send.mutate()
  }

  return (
    // noValidate: the server's message says what's wrong with an email more plainly than the browser's bubble.
    <form noValidate aria-label="Suggest a relative" className={styles.suggestForm} onSubmit={onSubmit}>
      <div className={styles.field}>
        <label htmlFor={`${id}-name`} className={styles.label}>
          Their full name
        </label>
        <input
          ref={nameRef}
          id={`${id}-name`}
          type="text"
          // Someone else's details: the browser would fill in yours.
          autoComplete="off"
          readOnly={send.isPending}
          value={fullName}
          onChange={(e) => {
            setFullName(e.target.value)
            setNoName(false)
          }}
          aria-invalid={noName || undefined}
          aria-describedby={noName ? `${id}-name-error` : undefined}
          className={styles.input}
        />
        {noName && (
          <span id={`${id}-name-error`} role="alert" className={styles.fieldError}>
            Enter their full name.
          </span>
        )}
      </div>
      <div className={styles.field}>
        <label htmlFor={`${id}-email`} className={styles.label}>
          Their email
        </label>
        <input
          id={`${id}-email`}
          type="email"
          autoComplete="off"
          readOnly={send.isPending}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className={styles.input}
        />
      </div>
      <fieldset className={styles.related}>
        <legend className={styles.label}>
          How are they related? <span className={styles.optional}>(optional)</span>
        </legend>
        <div className={styles.sentence}>
          <span>They’re the</span>
          <label htmlFor={`${id}-rel`} className="visually-hidden">
            Relationship
          </label>
          <select
            id={`${id}-rel`}
            value={relationship}
            disabled={send.isPending}
            onChange={(e) => setRelationship(e.target.value)}
            className={`${styles.input} ${styles.relation}`}
          >
            <option value="">—</option>
            {relationships.map((r) => (
              <option key={r}>{r}</option>
            ))}
          </select>
          <span>of</span>
          <label htmlFor={`${id}-of`} className="visually-hidden">
            Related to
          </label>
          <input
            id={`${id}-of`}
            type="text"
            readOnly={send.isPending}
            value={relatedTo}
            onChange={(e) => setRelatedTo(e.target.value)}
            className={`${styles.input} ${styles.relatedTo}`}
          />
        </div>
      </fieldset>
      <div className={styles.field}>
        <label htmlFor={`${id}-note`} className={styles.label}>
          Anything else? <span className={styles.optional}>(optional)</span>
        </label>
        <textarea
          id={`${id}-note`}
          rows={3}
          readOnly={send.isPending}
          value={note}
          onChange={(e) => setNote(e.target.value)}
          className={`${styles.input} ${styles.textarea}`}
        />
      </div>
      {send.isError && (
        <p role="alert" className={styles.formError}>
          {send.error instanceof ApiError && send.error.status === 403
            ? 'Your login isn’t linked to you in the tree yet, so you can’t suggest anyone. An admin can link it.'
            : send.error instanceof ApiError && send.error.status === 409
              ? 'Your login is now linked to someone else in the tree.'
              : refusedMessage(send.error)}
        </p>
      )}
      <div className={styles.actions}>
        <button type="submit" aria-disabled={send.isPending || undefined} className={`pill ${styles.save}`}>
          Send suggestion
        </button>
        {/* Held while it sends, so its answer, or its failure, is never missed. */}
        <button
          type="button"
          onClick={() => !send.isPending && onDone(null)}
          aria-disabled={send.isPending || undefined}
          className={`pill ${styles.cancel}`}
        >
          Cancel
        </button>
      </div>
    </form>
  )
}
