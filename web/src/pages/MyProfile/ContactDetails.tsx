import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useId, useState } from 'react'

import {
  addMyContact,
  contactKeys,
  type ContactMethod,
  directoryKeys,
  kindOf,
  myContactsQuery,
  type NewContact,
  platformLabel,
  removeMyContact,
  updateMyContact,
} from '../../api/contacts'
import { onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { errorMessage, refusedMessage } from '../../lib/errorMessage'
import { formatPhone } from '../../lib/phone'
import styles from './MyProfile.module.css'

type ContactType = {
  id: string
  name: string
  kind: ContactMethod['kind']
  platform: string | null
  hint: string
  // Kind "other" rows come only from an admin; members add an "Other account" instead.
  addable?: false
}

const social = (id: string, name = platformLabel(id)): ContactType => ({
  id,
  name,
  kind: 'social',
  platform: id,
  hint: '@handle or a link to your profile',
})

const types: ContactType[] = [
  { id: 'phone', name: 'Phone', kind: 'phone', platform: null, hint: '555-010-0000' },
  { id: 'email', name: 'Email', kind: 'email', platform: null, hint: 'name@example.com' },
  { id: 'address', name: 'Address', kind: 'address', platform: null, hint: '' },
  { ...social('whatsapp'), hint: '555-010-0000' },
  social('instagram'),
  social('facebook'),
  social('tiktok'),
  social('linkedin'),
  social('x'),
  social('youtube'),
  { ...social('other', 'Other account'), hint: 'Your handle, or a link' },
  { id: 'other-kind', name: 'Other', kind: 'other', platform: null, hint: '', addable: false },
]

const typeOf = (c: Pick<ContactMethod, 'kind' | 'platform'>) =>
  types.find((t) => t.kind === c.kind && t.platform === c.platform)

// The mockup's order, whatever order they were added in.
const rank = (c: ContactMethod) => {
  const type = typeOf(c)
  return type ? types.indexOf(type) : types.length
}
const byType = (a: ContactMethod, b: ContactMethod) => rank(a) - rank(b)

const typeName = (c: ContactMethod) =>
  c.kind === 'social' ? platformLabel(c.platform) : (typeOf(c)?.name ?? 'Other')

const askedFor = (c: ContactMethod) => kindOf(c) !== null

function useContactSaved(personId: string) {
  const queryClient = useQueryClient()
  return {
    saved: (update: (rows: ContactMethod[]) => ContactMethod[]) => {
      queryClient.setQueryData<ContactMethod[]>(contactKeys.mine(personId), (rows) => rows && update(rows))
      // Your own rows too: a refresh that read them before this save mustn't land after it.
      for (const queryKey of [contactKeys.mine(personId), contactKeys.of(personId), directoryKeys.all]) {
        void queryClient.invalidateQueries({ queryKey })
      }
    },
    onError: onSessionEnded(queryClient),
  }
}

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
      {contacts.isPending && <p className={styles.muted}>Loading…</p>}
      {contacts.isError && <p role="alert">{errorMessage(contacts.error)}</p>}
      {(contacts.data ? [...contacts.data].sort(byType) : []).map((c) =>
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
  const { saved, onError } = useContactSaved(personId)
  const on = contact.visibility !== 'hidden'
  const status = { hidden: 'Only me', on_request: 'Family can ask', family: 'Visible to family' }[contact.visibility]
  const toggle = useMutation({
    mutationFn: () => updateMyContact(contact.id, { visibility: on ? 'hidden' : askedFor(contact) ? 'on_request' : 'family' }),
    onMutate: sessionGeneration,
    onSuccess: (row, _, since) => sameSession(since) && saved((rows) => rows.map((r) => (r.id === row.id ? row : r))),
    onError,
  })

  return (
    <div className={styles.contact}>
      <div className={styles.contactText}>
        <span className={styles.labelLine}>
          <span id={`${id}-label`} className={styles.contactLabel}>
            {contact.label ?? typeName(contact)}
          </span>
          {/* Not while the switch saves: the two answers would race to update this row. */}
          <button
            type="button"
            onClick={() => !toggle.isPending && onEdit()}
            aria-disabled={toggle.isPending || undefined}
            aria-label={`Edit ${contact.label ?? typeName(contact)}`}
            className={styles.edit}
          >
            Edit
          </button>
        </span>
        <span className={styles.contactValue}>{contact.value}</span>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={on}
        aria-labelledby={`${id}-label`}
        aria-describedby={`${id}-status`}
        aria-disabled={toggle.isPending || undefined}
        onClick={() => !toggle.isPending && toggle.mutate()}
        className={styles.switch}
      >
        <span aria-hidden="true" className={styles.track}>
          <span className={styles.knob} />
        </span>
        <span id={`${id}-status`} className={styles.status}>
          {status}
        </span>
      </button>
      {toggle.isError && (
        <p role="alert" className={styles.rowError}>
          {refusedMessage(toggle.error)}
        </p>
      )}
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
