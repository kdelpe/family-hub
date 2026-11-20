import { useIsMutating, useMutation, useQuery } from '@tanstack/react-query'
import { type FormEvent, useId, useState } from 'react'

import {
  addMyContact,
  type ContactMethod,
  myContactsQuery,
  type NewContact,
  NotYoursError,
  RelinkedError,
  removeMyContact,
  updateMyContact,
} from '../../api/contacts'
import { sameSession, sessionGeneration } from '../../api/auth'
import { ContactSwitch, contactSwitchKey } from '../../components/ContactSwitch/ContactSwitch'
import { errorMessage, refusedMessage } from '../../lib/errorMessage'
import { byType, contactTypes as types, typeName, typeOf } from '../../lib/contactTypes'
import { formatPhone } from '../../lib/phone'
import { useContactSaved } from '../../lib/useContactSaved'
import styles from './MyProfile.module.css'

export function ContactDetails({ personId }: { personId: string }) {
  const contacts = useQuery(myContactsQuery(personId))
  const [adding, setAdding] = useState(false)
  // Each open editor stands alone, so opening another never closes one mid-save.
  const [editing, setEditing] = useState<ReadonlySet<string>>(new Set())
  const edit = (id: string, open: boolean) =>
    setEditing((ids) => {
      const next = new Set(ids)
      if (open) {
        next.add(id)
      } else {
        next.delete(id)
      }
      return next
    })

  return (
    <section aria-labelledby="mp-contact" className={styles.card}>
      <div className={styles.cardHeading}>
        <h2 id="mp-contact" className={styles.cardTitle}>
          Contact details
        </h2>
        <span className={styles.muted}>Nothing is shared until you turn it on.</span>
      </div>
      {(contacts.isPending || contacts.error instanceof RelinkedError) && <p className={styles.muted}>Loading…</p>}
      {contacts.isError && !(contacts.error instanceof RelinkedError) && <p role="alert">{errorMessage(contacts.error)}</p>}
      {(contacts.data && !(contacts.error instanceof NotYoursError) ? [...contacts.data].sort(byType) : []).map((c) =>
        editing.has(c.id) ? (
          <ContactForm key={c.id} personId={personId} contact={c} onDone={() => edit(c.id, false)} />
        ) : (
          <ContactRow key={c.id} personId={personId} contact={c} onEdit={() => edit(c.id, true)} />
        ),
      )}
      {adding ? (
        <ContactForm personId={personId} onDone={() => setAdding(false)} />
      ) : (
        contacts.isSuccess && (
          <button type="button" onClick={() => setAdding(true)} className={`pill ${styles.add}`}>
            + Add a phone, email, address or social account
          </button>
        )
      )}
    </section>
  )
}

function ContactRow({ personId, contact, onEdit }: { personId: string; contact: ContactMethod; onEdit: () => void }) {
  const id = useId()
  // Not while the switch saves: the two answers would race to update this row.
  const switching = useIsMutating({ mutationKey: contactSwitchKey(contact.id) }) > 0

  return (
    <div className={styles.contact}>
      <div className={styles.contactText}>
        <span className={styles.labelLine}>
          <span id={`${id}-label`} className={styles.contactLabel}>
            {contact.label ?? typeName(contact)}
          </span>
          <button
            type="button"
            onClick={() => !switching && onEdit()}
            aria-disabled={switching || undefined}
            aria-label={`Edit ${contact.label ?? typeName(contact)}`}
            className={styles.edit}
          >
            Edit
          </button>
        </span>
        <span className={styles.contactValue}>{contact.value}</span>
      </div>
      <ContactSwitch personId={personId} contact={contact} labelledBy={`${id}-label`} />
    </div>
  )
}

function ContactForm({ personId, contact, onDone }: { personId: string; contact?: ContactMethod; onDone: () => void }) {
  const id = useId()
  const { saved, onError } = useContactSaved(personId)
  const [typeId, setTypeId] = useState(contact ? (typeOf(contact)?.id ?? 'other-kind') : 'phone')
  const [label, setLabel] = useState(contact?.label ?? '')
  const [value, setValue] = useState(contact?.value ?? '')
  // A second step, since a removed row and who it was shared with can't be brought back.
  const [confirming, setConfirming] = useState(false)
  const type = types.find((t) => t.id === typeId) ?? types[0]!
  const isPhone = type.kind === 'phone' || type.platform === 'whatsapp'

  const save = useMutation({
    mutationFn: () => {
      const fields = { label: label.trim() || null, value }
      if (contact) {
        return updateMyContact(contact.id, fields)
      }
      // New rows start as "Only me" (design-notes §5, rule 6).
      const row: NewContact = { kind: type.kind, platform: type.platform, visibility: 'hidden', ...fields }
      return addMyContact(row)
    },
    onMutate: sessionGeneration,
    onSuccess: (row, _, since) => {
      if (sameSession(since)) {
        saved((rows) => (contact ? rows.map((r) => (r.id === row.id ? row : r)) : [...rows, row]))
        onDone()
      }
    },
    onError,
  })
  const remove = useMutation({
    mutationFn: () => removeMyContact(contact!.id),
    onMutate: sessionGeneration,
    onSuccess: (_, __, since) => {
      if (sameSession(since)) {
        saved((rows) => rows.filter((r) => r.id !== contact!.id))
        onDone()
      }
    },
    onError,
  })
  const busy = save.isPending || remove.isPending
  // Each try clears the other's error, so the one shown is always the latest.
  const failed = save.error ?? remove.error

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (busy) {
      return
    }
    if (contact && value === contact.value && (label.trim() || null) === contact.label) {
      onDone()
      return
    }
    remove.reset()
    save.mutate()
  }

  return (
    // noValidate: the server's message says what's wrong more plainly than the browser's bubble.
    <form noValidate className={styles.contactForm} onSubmit={onSubmit} aria-label={contact ? `Edit ${typeName(contact)}` : 'Add a contact'}>
      {!contact && (
        <div className={styles.field}>
          <label htmlFor={`${id}-type`} className={styles.label}>
            Type
          </label>
          <select
            id={`${id}-type`}
            value={typeId}
            disabled={busy}
            onChange={(e) => {
              setTypeId(e.target.value)
              setValue('')
              save.reset()
            }}
            className={styles.input}
          >
            {types.filter((t) => t.addable !== false).map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
        </div>
      )}
      <div className={styles.field}>
        <label htmlFor={`${id}-value`} className={styles.label}>
          {contact ? typeName(contact) : type.kind === 'social' ? 'Handle or link' : type.name}
        </label>
        {type.kind === 'address' ? (
          <textarea
            id={`${id}-value`}
            rows={3}
            required
            readOnly={busy}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className={`${styles.input} ${styles.textarea}`}
          />
        ) : (
          <input
            id={`${id}-value`}
            type={type.kind === 'email' ? 'email' : isPhone ? 'tel' : 'text'}
            required
            readOnly={busy}
            value={value}
            placeholder={type.hint}
            // Only while typing at the end: regrouping mid-number would throw the cursor to the end.
            onChange={(e) => {
              const atEnd = e.target.selectionStart === null || e.target.selectionStart === e.target.value.length
              setValue(isPhone && atEnd ? formatPhone(e.target.value) : e.target.value)
            }}
            className={styles.input}
          />
        )}
      </div>
      <div className={styles.field}>
        <label htmlFor={`${id}-label`} className={styles.label}>
          Label <span className={styles.optional}>(optional)</span>
        </label>
        <input
          id={`${id}-label`}
          readOnly={busy}
          value={label}
          placeholder={type.platform === 'other' ? 'Like Mastodon' : 'Like Mobile or Work'}
          onChange={(e) => setLabel(e.target.value)}
          className={styles.input}
        />
      </div>
      {failed && (
        <p role="alert" className={styles.formError}>
          {refusedMessage(failed)}
        </p>
      )}
      {confirming ? (
        <div role="group" aria-label="Remove this contact?" className={styles.actions}>
          <span className={styles.confirm}>Remove this {typeName(contact!).toLowerCase()}?</span>
          <button
            type="button"
            onClick={() => {
              if (!busy) {
                save.reset()
                remove.mutate()
              }
            }}
            aria-disabled={busy || undefined}
            className={`pill ${styles.save}`}
          >
            Yes, remove
          </button>
          <button
            type="button"
            onClick={() => !busy && setConfirming(false)}
            aria-disabled={busy || undefined}
            className={`pill ${styles.cancel}`}
          >
            Keep it
          </button>
        </div>
      ) : (
      <div className={styles.actions}>
        <button type="submit" aria-disabled={busy || undefined} className={`pill ${styles.save}`}>
          Save
        </button>
        {/* Held while a save runs, so its answer, or its failure, is never missed. */}
        <button
          type="button"
          onClick={() => !busy && onDone()}
          aria-disabled={busy || undefined}
          className={`pill ${styles.cancel}`}
        >
          Cancel
        </button>
        {contact && (
          <button type="button" onClick={() => !busy && setConfirming(true)} aria-disabled={busy || undefined} className={styles.remove}>
            Remove
          </button>
        )}
      </div>
      )}
    </form>
  )
}
