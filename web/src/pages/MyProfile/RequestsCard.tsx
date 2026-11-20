import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState } from 'react'

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

// The second step before an answer goes out, inside the row, like Remove on a contact.
function Confirm({
  question,
  note,
  yes,
  no,
  busy,
  onYes,
  onNo,
}: {
  question: string
  note?: string
  yes: string
  no: string
  busy: boolean
  onYes: () => void
  onNo: () => void
}) {
  const id = useId()
  const yesRef = useRef<HTMLButtonElement>(null)
  // Focus moves in, so the group's question and note are read out and the keyboard isn't left on a removed button.
  useEffect(() => yesRef.current?.focus(), [])
  return (
    <div
      role="group"
      aria-labelledby={`${id}-question`}
      aria-describedby={note ? `${id}-note` : undefined}
      className={styles.confirming}
    >
      <span id={`${id}-question`} className={styles.confirm}>
        {question}
      </span>
      {note && (
        <span id={`${id}-note`} className={styles.confirmNote}>
          {note}
        </span>
      )}
      <div className={styles.choices}>
        <button ref={yesRef} type="button" onClick={() => !busy && onYes()} aria-disabled={busy || undefined} className={`pill ${styles.save}`}>
          {yes}
        </button>
        <button type="button" onClick={() => !busy && onNo()} aria-disabled={busy || undefined} className={`pill ${styles.cancel}`}>
          {no}
        </button>
      </div>
    </div>
  )
}

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
  // Both ask first: a decline can't be taken back, and a share shows the detail at once (owner's call).
  const [confirming, setConfirming] = useState<'share' | 'decline' | null>(null)
  // After Cancel, back to the button that opened the question; after an answer, to the line saying what happened.
  const [returnTo, setReturnTo] = useState<'share' | 'decline' | null>(null)
  const shareRef = useRef<HTMLButtonElement>(null)
  const declineRef = useRef<HTMLButtonElement>(null)
  const answerRef = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    if (returnTo) {
      ;(returnTo === 'share' ? shareRef : declineRef).current?.focus()
    }
  }, [returnTo])
  useEffect(() => {
    if (answer) {
      answerRef.current?.focus()
    }
  }, [answer])
  const decide = useDecision(personId, request, (d) => onAnswered(d === 'share' ? 'shared' : 'declined'))
  const first = firstName(request.from.name)
  const detail = yourDetail[request.kind]

  return (
    <div className={styles.request}>
      <span>
        <strong>{request.from.name}</strong> asked for your {detail}.
      </span>
      {!answer &&
        (confirming ? (
          <Confirm
            question={confirming === 'share' ? `Share your ${detail} with ${first}?` : `Don’t share your ${detail} with ${first}?`}
            // A share covers every value of the kind, now and later (§5.2), not just the one they may have in mind.
            note={
              confirming === 'share'
                ? `${first} will see every ${detail} you’ve set to “Family can ask”, including ones you add later.`
                : 'You can’t change your mind later.'
            }
            yes={confirming === 'share' ? 'Yes, share' : 'Yes, don’t share'}
            no="Cancel"
            busy={decide.isPending}
            onYes={() => decide.mutate(confirming)}
            onNo={() => {
              setReturnTo(confirming)
              setConfirming(null)
              decide.reset()
            }}
          />
        ) : (
          <div className={styles.choices}>
            <button
              ref={shareRef}
              type="button"
              onClick={() => {
                setReturnTo(null)
                setConfirming('share')
              }}
              className={`pill ${styles.save}`}
            >
              Share
            </button>
            <button
              ref={declineRef}
              type="button"
              onClick={() => {
                setReturnTo(null)
                setConfirming('decline')
              }}
              className={`pill ${styles.cancel}`}
            >
              Don’t share
            </button>
          </div>
        ))}
      {/* Mounted before the answer, so screen readers announce it. */}
      <span ref={answerRef} tabIndex={-1} role="status" className={styles.answer}>
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
  const [kept, setKept] = useState(false)
  const stopRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (kept) {
      stopRef.current?.focus()
    }
  }, [kept])
  const stop = useDecision(personId, request, onStopped)
  const detail = yourDetail[request.kind]

  return (
    <li className={styles.sharedLine}>
      <span>
        {request.from.name} · {detail}
      </span>
      {confirming ? (
        <Confirm
          question={`Stop sharing your ${detail} with ${firstName(request.from.name)}?`}
          yes="Yes, stop"
          no="Keep sharing"
          busy={stop.isPending}
          onYes={() => stop.mutate('revoke')}
          onNo={() => {
            setKept(true)
            setConfirming(false)
            stop.reset()
          }}
        />
      ) : (
        <button
          ref={stopRef}
          type="button"
          onClick={() => {
            setKept(false)
            setConfirming(true)
          }}
          className={styles.remove}
        >
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
