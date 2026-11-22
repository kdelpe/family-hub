import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { type ContactRequest, myRequestsQuery, NotYoursError, RelinkedError } from '../../api/contacts'
import { errorMessage } from '../../lib/errorMessage'
import { useSignedIn } from '../../lib/useSignedIn'
import { type Answer, RequestRow, SharedLine, yourDetail } from '../MyProfile/RequestsCard'
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
        <RequestTabs personId={personId} />
      ) : (
        <p className={styles.muted}>Your login isn’t linked to you in the tree yet, so nobody can ask for your details.</p>
      )}
    </main>
  )
}

function RequestTabs({ personId }: { personId: string }) {
  const requests = useQuery(myRequestsQuery(personId))
  const [tab, setTab] = useState<Tab>('waiting')
  // Answered on this visit: each stays in Waiting, saying what happened, until the page is left.
  const [answered, setAnswered] = useState<{ request: ContactRequest; answer: Answer }[]>([])
  const rows = requests.error instanceof NotYoursError ? [] : (requests.data ?? [])
  const justAnswered = (r: ContactRequest) => answered.find((a) => a.request.id === r.id)
  const waiting = rows.filter((r) => r.status === 'pending' || justAnswered(r))
  const past = rows.filter((r) => r.status !== 'pending' && !justAnswered(r)).reverse()
  const shared = rows.filter((r) => r.status === 'shared')
  const count = rows.filter((r) => r.status === 'pending').length

  const tabs: { id: Tab; label: string }[] = [
    { id: 'waiting', label: 'Waiting' },
    { id: 'answered', label: 'Answered' },
    { id: 'shared', label: 'Shared with' },
  ]

  let body
  if (requests.isPending || requests.error instanceof RelinkedError) {
    body = <p className={styles.muted}>Loading…</p>
  } else if (requests.isError) {
    body = <p role="alert">{errorMessage(requests.error)}</p>
  } else if (tab === 'waiting') {
    body = waiting.length ? (
      <div className={styles.list}>
        {waiting.map((request) => (
          <RequestRow
            key={request.id}
            personId={personId}
            request={request}
            answer={justAnswered(request)?.answer}
            onAnswered={(a) => setAnswered((list) => [...list, { request, answer: a }])}
          />
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
          <SharedLine key={request.id} personId={personId} request={request} onStopped={() => {}} />
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
        {body}
      </div>
      <p className={styles.note}>
        Sharing shows that one person every detail of that kind you’ve set to “Family can ask”, including ones you add
        later. You can stop sharing any time.
      </p>
    </>
  )
}
