import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'

import { onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { ApiError } from '../../api/client'
import {
  type ContactRequest,
  contactRequestKeys,
  decideRequest,
  myRequestsQuery,
  NotYoursError,
  RelinkedError,
  type RequestKind,
} from '../../api/contacts'
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog'
import { errorMessage } from '../../lib/errorMessage'
import styles from './MyProfile.module.css'

export const yourDetail: Record<RequestKind, string> = { phone: 'phone number', whatsapp: 'WhatsApp number', address: 'address' }

// As in the mockup: a request names them in full, its answer by first name.
export const firstName = (name: string) => name.split(' ')[0] ?? name

export type Answer = 'shared' | 'declined'

const oldestFirst = (a: { request: ContactRequest }, b: { request: ContactRequest }) =>
  Date.parse(a.request.created_at) - Date.parse(b.request.created_at)

// design-notes §4.8: who asked for your details, then who you've shared them with.
export function RequestsCard({ personId }: { personId: string }) {
  const requests = useQuery(myRequestsQuery(personId))
  // Answered on this visit: each stays where it was, saying what happened.
  const [answered, setAnswered] = useState<{ request: ContactRequest; answer: Answer }[]>([])
  const [stopped, setStopped] = useState<ContactRequest | null>(null)
  // The shared line it was on is gone, so keyboard focus lands on what happened.
  const stoppedRef = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    if (stopped) {
      stoppedRef.current?.focus()
    }
  }, [stopped])

  // Rows kept from before a relink are someone else's: none are shown, so none can be answered.
  const rows = requests.error instanceof NotYoursError ? [] : (requests.data ?? [])
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
      {(requests.isPending || requests.error instanceof RelinkedError) && <p className={styles.muted}>Loading…</p>}
      {requests.isError && !(requests.error instanceof RelinkedError) && <p role="alert">{errorMessage(requests.error)}</p>}
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
              <SharedLine key={request.id} personId={personId} request={request} onStopped={() => {
                  setStopped(request)
                  // A share given on this visit and then taken back no longer says "Shared with".
                  setAnswered((list) => list.filter((a) => a.request.id !== request.id))
                }}
              />
            ))}
          </ul>
        </div>
      )}
      <span ref={stoppedRef} tabIndex={-1} role="status" className={styles.answer}>
        {stopped && `Stopped sharing your ${yourDetail[stopped.kind]} with ${firstName(stopped.from.name)}.`}
      </span>
      <span className={styles.small}>
        Sharing shows that one person every detail of that kind you’ve set to “Family can ask”, including ones you add
        later. You can stop sharing any time.
      </span>
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
      // Answered, it moves to Shared with or Answered.
      const status = ({ share: 'shared', decline: 'declined', revoke: 'revoked' } as const)[decision]
      // Dated now, so Answered sorts it newest first even if the reread that follows fails.
      const decided_at = new Date().toISOString()
      queryClient.setQueryData<ContactRequest[]>(queryKey, (rows) => rows?.map((r) => (r.id === request.id ? { ...r, status, decided_at } : r)))
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

export function RequestRow({
  personId,
  request,
  answer,
  onAnswered,
  meta,
}: {
  personId: string
  request: ContactRequest
  answer: Answer | undefined
  onAnswered: (answer: Answer) => void
  meta?: string
}) {
  // Both ask first: a decline can't be taken back, and a share shows the detail at once (owner's call).
  const [confirming, setConfirming] = useState<'share' | 'decline' | null>(null)
  const answerRef = useRef<HTMLSpanElement>(null)
  // Once answered, the buttons that opened the question are gone: focus goes to the line saying what happened.
  const answered = useRef(false)
  const decide = useDecision(personId, request, (d) => {
    answered.current = true
    setConfirming(null)
    onAnswered(d === 'share' ? 'shared' : 'declined')
  })
  const first = firstName(request.from.name)
  const detail = yourDetail[request.kind]

  return (
    <div className={styles.request}>
      <span>
        <strong>{request.from.name}</strong> asked for your {detail}.
        {meta && <span className={styles.requestMeta}>{meta}</span>}
      </span>
      {!answer && (
        <div className={styles.choices}>
          <button type="button" onClick={() => setConfirming('share')} className={`pill ${styles.save}`}>
            Share
          </button>
          <button type="button" onClick={() => setConfirming('decline')} className={`pill ${styles.cancel}`}>
            Don’t share
          </button>
        </div>
      )}
      {/* Mounted before the answer, so screen readers announce it. */}
      <span ref={answerRef} tabIndex={-1} role="status" className={styles.answer}>
        {answer === 'shared' ? `Shared with ${first}.` : answer === 'declined' ? `Not shared. ${first} won’t be told.` : ''}
      </span>
      <ConfirmDialog
        open={confirming !== null && !answer}
        question={confirming === 'decline' ? `Don’t share your ${detail} with ${first}?` : `Share your ${detail} with ${first}?`}
        // A share covers every value of the kind, now and later (§5.2), not just the one they may have in mind.
        note={
          confirming === 'decline'
            ? 'You can’t change your mind later.'
            : `${first} will see every ${detail} you’ve set to “Family can ask”, including ones you add later.`
        }
        yes={confirming === 'decline' ? 'Yes, don’t share' : 'Yes, share'}
        no="Cancel"
        busy={decide.isPending}
        error={decide.isError ? decisionError(decide.error) : undefined}
        onYes={() => confirming && decide.mutate(confirming)}
        onCancel={() => {
          setConfirming(null)
          decide.reset()
        }}
        focusAfterClose={() => (answered.current ? answerRef.current : null)}
      />
    </div>
  )
}

export function SharedLine({ personId, request, onStopped }: { personId: string; request: ContactRequest; onStopped: () => void }) {
  // A second step: once taken back, a share can't be given again, and they can't ask twice.
  const [confirming, setConfirming] = useState(false)
  const stop = useDecision(personId, request, onStopped)
  const detail = yourDetail[request.kind]

  return (
    <li className={styles.sharedLine}>
      <span>
        {request.from.name} · {detail}
      </span>
      <button type="button" onClick={() => setConfirming(true)} className={styles.remove}>
        Stop sharing
      </button>
      <ConfirmDialog
        open={confirming}
        question={`Stop sharing your ${detail} with ${firstName(request.from.name)}?`}
        yes="Yes, stop"
        no="Keep sharing"
        busy={stop.isPending}
        error={stop.isError ? decisionError(stop.error) : undefined}
        onYes={() => stop.mutate('revoke')}
        onCancel={() => {
          setConfirming(false)
          stop.reset()
        }}
      />
    </li>
  )
}
