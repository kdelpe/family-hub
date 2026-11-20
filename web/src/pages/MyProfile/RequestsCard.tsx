import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { ApiError } from '../../api/client'
import {
  type ContactRequest,
  contactRequestKeys,
  decideRequest,
  myRequestsQuery,
  type RequestKind,
} from '../../api/contacts'
import { errorMessage } from '../../lib/errorMessage'
import styles from './MyProfile.module.css'

const yourDetail: Record<RequestKind, string> = { phone: 'phone number', whatsapp: 'WhatsApp number', address: 'address' }

// As in the mockup: a request names them in full, its answer by first name.
const firstName = (name: string) => name.split(' ')[0] ?? name

type Answer = 'shared' | 'declined'

const oldestFirst = (a: { request: ContactRequest }, b: { request: ContactRequest }) =>
  Date.parse(a.request.created_at) - Date.parse(b.request.created_at)

// design-notes §4.8: who asked for your details, then who you've shared them with.
export function RequestsCard({ personId }: { personId: string }) {
  const requests = useQuery(myRequestsQuery(personId))
  // Answered on this visit: each stays where it was, saying what happened.
  const [answered, setAnswered] = useState<{ request: ContactRequest; answer: Answer }[]>([])
  const [stopped, setStopped] = useState<ContactRequest | null>(null)

  const rows = requests.data ?? []
  const asked = [
    ...rows
      .filter((r) => r.status === 'pending' && !answered.some((a) => a.request.id === r.id))
      .map((request) => ({ request, answer: undefined })),
    ...answered,
  ].sort(oldestFirst)
  const shared = rows.filter((r) => r.status === 'shared')

  return (
    <section aria-labelledby="mp-requests" className={`${styles.side} ${styles.requests}`}>
      <h2 id="mp-requests" className={styles.sideTitle}>
        Requests for your details
      </h2>
      {requests.isPending && <p className={styles.muted}>Loading…</p>}
      {requests.isError && <p role="alert">{errorMessage(requests.error)}</p>}
      {requests.isSuccess && asked.length === 0 && <p className={styles.muted}>No requests right now.</p>}
      {asked.map(({ request, answer }) => (
        <RequestRow
          key={request.id}
          personId={personId}
          request={request}
          answer={answer}
          onAnswered={(a) => setAnswered((list) => [...list, { request, answer: a }])}
        />
      ))}
      {shared.length > 0 && (
        <div className={styles.sharedWith}>
          <h3 className={styles.sharedTitle}>Shared with</h3>
          <ul className={styles.sharedList}>
            {shared.map((request) => (
              <SharedLine key={request.id} personId={personId} request={request} onStopped={() => setStopped(request)} />
            ))}
          </ul>
        </div>
      )}
      <span role="status" className={styles.answer}>
        {stopped && `Stopped sharing your ${yourDetail[stopped.kind]} with ${firstName(stopped.from.name)}.`}
      </span>
      <span className={styles.small}>Sharing shows it to that one person only. You can stop sharing any time.</span>
    </section>
  )
}

function useDecision(personId: string, request: ContactRequest, onDecided: (decision: 'share' | 'decline' | 'revoke') => void) {
  const queryClient = useQueryClient()
  const queryKey = contactRequestKeys.mine(personId)
  return useMutation({
    mutationFn: (decision: 'share' | 'decline' | 'revoke') => decideRequest(request.id, decision),
    onMutate: sessionGeneration,
    onSuccess: (_, decision, since) => {
      if (!sameSession(since)) {
        return
      }
      onDecided(decision)
      // A share moves under Shared with; a decline or a revoke is gone for good.
      queryClient.setQueryData<ContactRequest[]>(queryKey, (rows) =>
        rows?.flatMap((r) => (r.id !== request.id ? [r] : decision === 'share' ? [{ ...r, status: 'shared' as const }] : [])),
      )
      void queryClient.invalidateQueries({ queryKey })
    },
    onError: (error, decision, since) => {
      onSessionEnded(queryClient)(error, decision, since)
      // Answered meanwhile, maybe in another tab: show where it stands now.
      if (error instanceof ApiError && error.status === 409) {
        void queryClient.invalidateQueries({ queryKey })
      }
    },
  })
}

const decisionError = (error: Error) =>
  error instanceof ApiError && error.status === 409 ? 'This was already answered.' : errorMessage(error)

function RequestRow({
  personId,
  request,
  answer,
  onAnswered,
}: {
  personId: string
  request: ContactRequest
  answer: Answer | undefined
  onAnswered: (answer: Answer) => void
}) {
  const decide = useDecision(personId, request, (d) => onAnswered(d === 'share' ? 'shared' : 'declined'))
  const first = firstName(request.from.name)
  const choose = (decision: 'share' | 'decline') => {
    if (!decide.isPending) {
      decide.mutate(decision)
    }
  }

  return (
    <div className={styles.request}>
      <span>
        <strong>{request.from.name}</strong> asked for your {yourDetail[request.kind]}.
      </span>
      {!answer && (
        <div className={styles.choices}>
          <button
            type="button"
            onClick={() => choose('share')}
            aria-disabled={decide.isPending || undefined}
            className={`pill ${styles.save}`}
          >
            Share
          </button>
          <button
            type="button"
            onClick={() => choose('decline')}
            aria-disabled={decide.isPending || undefined}
            className={`pill ${styles.cancel}`}
          >
            Not now
          </button>
        </div>
      )}
      {/* Mounted before the answer, so screen readers announce it. */}
      <span role="status" className={styles.answer}>
        {answer === 'shared' ? `Shared with ${first}.` : answer === 'declined' ? `Not shared. ${first} won’t be told.` : ''}
      </span>
      {decide.isError && (
        <p role="alert" className={styles.formError}>
          {decisionError(decide.error)}
        </p>
      )}
    </div>
  )
}

function SharedLine({ personId, request, onStopped }: { personId: string; request: ContactRequest; onStopped: () => void }) {
  // A second step: once taken back, a share can't be given again, and they can't ask twice.
  const [confirming, setConfirming] = useState(false)
  const stop = useDecision(personId, request, onStopped)
  const detail = yourDetail[request.kind]

  return (
    <li className={styles.sharedLine}>
      <span>
        {request.from.name} · {detail}
      </span>
      {confirming ? (
        <div role="group" aria-label="Stop sharing?" className={styles.stopping}>
          <span className={styles.confirm}>
            Stop sharing your {detail} with {firstName(request.from.name)}?
          </span>
          <div className={styles.choices}>
            <button
              type="button"
              onClick={() => !stop.isPending && stop.mutate('revoke')}
              aria-disabled={stop.isPending || undefined}
              className={`pill ${styles.save}`}
            >
              Yes, stop
            </button>
            <button
              type="button"
              onClick={() => !stop.isPending && setConfirming(false)}
              aria-disabled={stop.isPending || undefined}
              className={`pill ${styles.cancel}`}
            >
              Keep sharing
            </button>
          </div>
        </div>
      ) : (
        <button type="button" onClick={() => setConfirming(true)} className={styles.remove}>
          Stop sharing
        </button>
      )}
      {stop.isError && (
        <p role="alert" className={styles.formError}>
          {decisionError(stop.error)}
        </p>
      )}
    </li>
  )
}
