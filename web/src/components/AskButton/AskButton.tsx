import { useMutation, useQueryClient } from '@tanstack/react-query'

import { ApiError } from '../../api/client'
import {
  askFor,
  contactKeys,
  type ContactMethod,
  type DirectoryEntry,
  directoryKeys,
  isRequestable,
  kindOf,
  kindWords,
  type RequestKind,
} from '../../api/contacts'
import { errorMessage } from '../../lib/errorMessage'
import { CheckPill } from '../CheckPill/CheckPill'
import styles from './AskButton.module.css'

const askLabels: Record<RequestKind, string> = {
  phone: 'Request phone number',
  whatsapp: 'Request WhatsApp',
  address: 'Request address',
}

// The dashed "Request phone number" button (design-notes §4.3), which turns into "✓ … requested" once asked.
export function AskButton({
  personId,
  kind,
  asked,
  size,
}: {
  personId: string
  kind: RequestKind
  asked: boolean
  size: 'large' | 'small'
}) {
  const queryClient = useQueryClient()
  const ask = useMutation({
    mutationFn: () => askFor(personId, kind),
    // Not awaited: the ask is done, and a failed reload mustn't hold or undo it.
    onSuccess: ({ my_request_status }) => {
      void queryClient.invalidateQueries({ queryKey: contactKeys.of(personId) })
      if (my_request_status === 'shared') {
        void queryClient.invalidateQueries({ queryKey: directoryKeys.all })
        return
      }
      // The directory holds the whole family, so only this one entry changes rather than all of it being fetched again.
      queryClient.setQueryData<DirectoryEntry[]>(directoryKeys.all, (people) =>
        people?.map((p) =>
          p.id !== personId
            ? p
            : { ...p, contacts: p.contacts.map((c) => (isRequestable(c) && c.kind === kind ? { ...c, my_request_status: 'requested' } : c)) },
        ),
      )
    },
  })
  if (ask.data?.my_request_status === 'shared') {
    // Already shared: the refetched contacts show it instead.
    return null
  }
  const done = `${kindWords[kind]} requested`
  return (
    <>
      {/* Mounted before the ask, so screen readers announce the change. */}
      <span role="status" className="visually-hidden">
        {ask.isSuccess ? done : ''}
      </span>
      {asked || ask.isSuccess ? (
        <CheckPill size={size}>{done}</CheckPill>
      ) : (
        <span className={styles.ask}>
          <button
            type="button"
            className={`pill ${styles.button} ${size === 'large' ? styles.large : `tap ${styles.small}`}`}
            aria-disabled={ask.isPending}
            onClick={() => {
              if (!ask.isPending) {
                ask.mutate()
              }
            }}
          >
            {askLabels[kind]}
          </button>
          {ask.error && (
            <span role="alert" className={styles.error}>
              {ask.error instanceof ApiError && ask.error.status === 403
                ? 'Your login isn’t linked to you in the tree yet, so you can’t ask. An admin can link it.'
                : errorMessage(ask.error)}
            </span>
          )}
        </span>
      )}
    </>
  )
}

// A phone, WhatsApp or address its owner shared with you after you asked.
export function SharedValue({ contact }: { contact: ContactMethod }) {
  const kind = kindOf(contact)
  return (
    <span className={styles.shared}>
      {contact.label ?? (kind === 'phone' ? 'Phone' : kindWords[kind ?? 'address'])}: {contact.value}
    </span>
  )
}
