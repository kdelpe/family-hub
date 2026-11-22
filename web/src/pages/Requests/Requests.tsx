import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'

import { type ContactRequest, myRequestsQuery, NotYoursError, RelinkedError } from '../../api/contacts'
import { treeQuery } from '../../api/tree'
import { addedOn } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { initials } from '../../lib/names'
import { relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import { type Answer, firstName, RequestRow, SharedLine, yourDetail } from '../MyProfile/RequestsCard'
import styles from './Requests.module.css'

type Tab = 'waiting' | 'answered' | 'shared'

const answeredAs: Partial<Record<ContactRequest['status'], string>> = {
  shared: '✓ Shared',
  declined: 'Not shared',
  revoked: 'Stopped sharing',
}

// design-notes §4.10: requests for your details, on their own page (owner's pick R1, 2026-11-21).
export function Requests() {
  const { user } = useSignedIn()
  const personId = user?.person_id ?? null
  return (
    <main className={styles.page}>
      <title>Requests — Pluviose Family Hub</title>
      <h1 className={styles.title}>Requests</h1>
      <p className={styles.sub}>Family asking for your phone number, WhatsApp or address. They only see it if you say yes.</p>
      {personId ? (
        // Keyed by person: answers given for one person never stay for another after a relink.
        <RequestTabs key={personId} personId={personId} />
      ) : (
        <p className={styles.muted}>Your login isn’t linked to you in the tree yet, so nobody can ask for your details.</p>
      )}
    </main>
  )
}

// Their initials, from the name the request carries.
const face = (name: string) => {
  const [first = '', ...rest] = name.split(' ')
  return initials({ first_name: first, last_name: rest.at(-1) ?? null })
}

function RequestTabs({ personId, now = new Date() }: { personId: string; now?: Date }) {
  const requests = useQuery(myRequestsQuery(personId))
  const tree = useQuery(treeQuery)
  const relations = useMemo(() => (tree.data ? relationsTo(tree.data, personId) : null), [tree.data, personId])
  const meta = (r: ContactRequest) => {
    const asked = `asked ${addedOn(r.created_at, now)}`
    const relation = relations?.get(r.from.person_id)
    return relation ? `${relation} · ${asked}` : asked[0]!.toUpperCase() + asked.slice(1)
  }
  const [tab, setTab] = useState<Tab>('waiting')
  // Answered on this visit: each stays in Waiting, saying what happened, until the page is left.
  const [answered, setAnswered] = useState<{ request: ContactRequest; answer: Answer }[]>([])
  const [stopped, setStopped] = useState<ContactRequest | null>(null)
  // The shared line it was on is gone, so keyboard focus lands on what happened.
  const stoppedRef = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    if (stopped) {
      stoppedRef.current?.focus()
    }
  }, [stopped])
  const rows = requests.error instanceof NotYoursError ? [] : (requests.data ?? [])
  const justAnswered = (r: ContactRequest) => answered.find((a) => a.request.id === r.id)
  // Kept from this visit's answers, not the list, so a reread that fails or drops one doesn't take it away.
  const waiting = [...rows.filter((r) => r.status === 'pending' && !justAnswered(r)), ...answered.map((a) => a.request)].sort(
    (a, b) => Date.parse(a.created_at) - Date.parse(b.created_at),
  )
  // Newest answer first.
  const decided = (r: ContactRequest) => Date.parse(r.decided_at ?? r.created_at)
  const past = rows.filter((r) => r.status !== 'pending' && !justAnswered(r)).sort((a, b) => decided(b) - decided(a))
  const shared = rows.filter((r) => r.status === 'shared')
  const count = rows.filter((r) => r.status === 'pending').length

  const tabs: { id: Tab; label: string }[] = [
    { id: 'waiting', label: 'Waiting' },
    { id: 'answered', label: 'Answered' },
    { id: 'shared', label: 'Shared with' },
  ]

  let body
  // Rows kept from before a relink are someone else's: nothing shows, this visit's answers included, until it's settled.
  if (requests.isPending || requests.error instanceof RelinkedError || requests.error instanceof NotYoursError) {
    body = <p className={styles.muted}>Loading…</p>
  } else if (requests.isError && !requests.data) {
    // A failed reread keeps the list it had.
    body = <p role="alert">{errorMessage(requests.error)}</p>
  } else if (tab === 'waiting') {
    body = waiting.length ? (
      <div className={styles.list}>
        {waiting.map((request) => (
          <div key={request.id} className={styles.withFace}>
            <span aria-hidden="true" className={styles.face}>
              {face(request.from.name)}
            </span>
            <RequestRow
              personId={personId}
              request={request}
              answer={justAnswered(request)?.answer}
              onAnswered={(a) => setAnswered((list) => [...list, { request, answer: a }])}
              meta={meta(request)}
            />
          </div>
        ))}
      </div>
    ) : (
      <p className={styles.muted}>No requests right now.</p>
    )
  } else if (tab === 'answered') {
    body = past.length ? (
      <ul className={styles.list}>
        {past.map((r) => (
          <li key={r.id} className={styles.row}>
            <span>
              <strong>{r.from.name}</strong> asked for your {yourDetail[r.kind]}.
            </span>
            <span className={styles.status}>{answeredAs[r.status]}</span>
          </li>
        ))}
      </ul>
    ) : (
      <p className={styles.muted}>Nothing answered yet.</p>
    )
  } else {
    body = shared.length ? (
      <ul className={styles.list}>
        {shared.map((request) => (
          <SharedLine
            key={request.id}
            personId={personId}
            request={request}
            onStopped={() => {
              setStopped(request)
              // A share given on this visit and then taken back no longer says "Shared with".
              setAnswered((list) => list.filter((a) => a.request.id !== request.id))
            }}
          />
        ))}
      </ul>
    ) : (
      <p className={styles.muted}>You haven’t shared your details with anyone.</p>
    )
  }

  return (
    <>
      <div role="tablist" aria-label="Requests" className={styles.tabs}>
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            id={`requests-tab-${t.id}`}
            aria-selected={tab === t.id}
            aria-controls="requests-panel"
            aria-label={t.id === 'waiting' && count > 0 ? `Waiting, ${count} ${count === 1 ? 'request' : 'requests'}` : undefined}
            onClick={() => setTab(t.id)}
            className={tab === t.id ? `${styles.tab} ${styles.on}` : styles.tab}
          >
            {t.label}
            {t.id === 'waiting' && count > 0 && (
              <span aria-hidden="true" className={styles.count}>
                {count}
              </span>
            )}
          </button>
        ))}
      </div>
      <div role="tabpanel" id="requests-panel" aria-labelledby={`requests-tab-${tab}`} className={styles.panel}>
        {/* A failed reread keeps the rows it had, saying so. */}
        {requests.isError && requests.data && !(requests.error instanceof NotYoursError) && !(requests.error instanceof RelinkedError) && (
          <p role="alert">{errorMessage(requests.error)}</p>
        )}
        {body}
        <span ref={stoppedRef} tabIndex={-1} role="status" className={styles.stopped}>
          {stopped && `Stopped sharing your ${yourDetail[stopped.kind]} with ${firstName(stopped.from.name)}.`}
        </span>
      </div>
      <p className={styles.note}>
        Sharing shows that one person every detail of that kind you’ve set to “Family can ask”, including ones you add
        later. You can stop sharing any time.
      </p>
    </>
  )
}
