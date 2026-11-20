import { useMutation } from '@tanstack/react-query'

import { sameSession, sessionGeneration } from '../../api/auth'
import { type ContactMethod, kindOf, updateMyContact } from '../../api/contacts'
import { refusedMessage } from '../../lib/errorMessage'
import { useContactSaved } from '../../lib/useContactSaved'
import styles from './ContactSwitch.module.css'

// Lets other controls wait while a switch saves (useIsMutating): one row's, or with no id, any row's.
export const contactSwitchKey = (contactId?: string) => (contactId ? (['contact-switch', contactId] as const) : (['contact-switch'] as const))

const statuses = { hidden: 'Only me', on_request: 'Family can ask', family: 'Visible to family' }

// One of your own contact rows: "Only me", or on for the family. A phone, WhatsApp or address is then only
// asked for, never shown (§5.2). Saves as soon as it's flipped. My profile and First setup share it.
export function ContactSwitch({
  personId,
  contact,
  labelledBy,
  size = 'regular',
}: {
  personId: string
  contact: ContactMethod
  labelledBy: string
  // First setup's mockup draws it a little tighter than My profile's.
  size?: 'regular' | 'small'
}) {
  const { saved, onError } = useContactSaved(personId)
  const on = contact.visibility !== 'hidden'
  const toggle = useMutation({
    mutationKey: contactSwitchKey(contact.id),
    mutationFn: () => updateMyContact(contact.id, { visibility: on ? 'hidden' : kindOf(contact) ? 'on_request' : 'family' }),
    onMutate: sessionGeneration,
    onSuccess: (row, _, since) => sameSession(since) && saved((rows) => rows.map((r) => (r.id === row.id ? row : r))),
    onError,
  })
  const statusId = `${labelledBy}-status`

  return (
    <>
      <button
        type="button"
        role="switch"
        aria-checked={on}
        aria-labelledby={labelledBy}
        aria-describedby={statusId}
        aria-disabled={toggle.isPending || undefined}
        onClick={() => !toggle.isPending && toggle.mutate()}
        className={size === 'small' ? `${styles.switch} ${styles.small}` : styles.switch}
      >
        <span aria-hidden="true" className={styles.track}>
          <span className={styles.knob} />
        </span>
        <span id={statusId} className={styles.status}>
          {statuses[contact.visibility]}
        </span>
      </button>
      {toggle.isError && (
        <p role="alert" className={styles.error}>
          {refusedMessage(toggle.error)}
        </p>
      )}
    </>
  )
}
