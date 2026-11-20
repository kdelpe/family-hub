import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Navigate } from 'react-router'

import { type AccessRequestItem, adminKeys, listAccessRequests, listMembers } from '../../api/admin'
import { useSignedIn } from '../../lib/useSignedIn'
import { Everyone } from './Everyone'
import { type Approval, InviteForm } from './InviteForm'
import styles from './ManageAccess.module.css'
import { Requests } from './Requests'

type Tab = 'requests' | 'invite' | 'everyone'

// Members & invites (design-notes §4.10).
export function ManageAccess() {
  const { user } = useSignedIn()
  const [tab, setTab] = useState<Tab>('requests')
  const [approval, setApproval] = useState<Approval | null>(null)
  const members = useQuery({
    queryKey: adminKeys.members,
    queryFn: ({ signal }) => listMembers({ signal }),
    enabled: user?.role === 'admin',
  })
  const requests = useQuery({
    queryKey: adminKeys.requests,
    queryFn: ({ signal }) => listAccessRequests({ signal }),
    enabled: user?.role === 'admin',
  })

  if (user && user.role !== 'admin') {
    return <Navigate to="/" replace />
  }

  const waiting = requests.data?.filter((r) => r.status === 'pending').length
  const approve = (request: AccessRequestItem) => {
    setApproval((current) => ({ request, n: (current?.n ?? 0) + 1 }))
    setTab('invite')
  }

  const tabs: { id: Tab; label: string }[] = [
    { id: 'requests', label: waiting === undefined ? 'Requests' : `Requests (${waiting})` },
    { id: 'invite', label: 'Invite someone' },
    { id: 'everyone', label: members.data ? `Everyone (${members.data.length})` : 'Everyone' },
  ]

  return (
    <main className={styles.page}>
      <title>Members & invites — Pluviose-Monestime Family Hub</title>
      <div className={styles.heading}>
        <h1 className={styles.title}>Family members &amp; invitations</h1>
        <span className={styles.muted}>Only admins see this page.</span>
      </div>

      <div role="tablist" aria-label="Sections" className={styles.tabs}>
        {tabs.map(({ id, label }) => (
          <button
            key={id}
            type="button"
            role="tab"
            id={`ma-tab-${id}`}
            aria-selected={tab === id}
            aria-controls={`ma-panel-${id}`}
            tabIndex={tab === id ? 0 : -1}
            onKeyDown={(event) => {
              const next = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0
              if (next !== 0) {
                const to = tabs[(tabs.findIndex((t) => t.id === tab) + next + tabs.length) % tabs.length]!.id
                setTab(to)
                document.getElementById(`ma-tab-${to}`)?.focus()
              }
            }}
            className={tab === id ? `${styles.tab} ${styles.tabOn}` : styles.tab}
            onClick={() => setTab(id)}
          >
            {label}
          </button>
        ))}
      </div>

      {/* All stay mounted, so an invite or resend still sending isn't lost by switching tabs. */}
      <div role="tabpanel" id="ma-panel-requests" aria-labelledby="ma-tab-requests" hidden={tab !== 'requests'}>
        <Requests requests={requests} onApprove={approve} />
      </div>
      <div role="tabpanel" id="ma-panel-invite" aria-labelledby="ma-tab-invite" hidden={tab !== 'invite'}>
        <InviteForm members={members} approval={approval} />
      </div>
      <div role="tabpanel" id="ma-panel-everyone" aria-labelledby="ma-tab-everyone" hidden={tab !== 'everyone'}>
        <Everyone members={members} />
      </div>
    </main>
  )
}
