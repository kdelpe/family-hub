import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Navigate } from 'react-router'

import { adminKeys, listMembers } from '../../api/admin'
import { useSignedIn } from '../../lib/useSignedIn'
import { Everyone } from './Everyone'
import { InviteForm } from './InviteForm'
import styles from './ManageAccess.module.css'

type Tab = 'invite' | 'everyone'

// Members & invites (design-notes §4.10). The Requests tab comes with the requests queue.
export function ManageAccess() {
  const { user } = useSignedIn()
  const [tab, setTab] = useState<Tab>('invite')
  const members = useQuery({
    queryKey: adminKeys.members,
    queryFn: ({ signal }) => listMembers({ signal }),
    enabled: user?.role === 'admin',
  })

  if (user && user.role !== 'admin') {
    return <Navigate to="/" replace />
  }

  const tabs: { id: Tab; label: string }[] = [
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
            aria-controls={tab === id ? `ma-panel-${id}` : undefined}
            tabIndex={tab === id ? 0 : -1}
            onKeyDown={(event) => {
              const next = { ArrowRight: 1, ArrowLeft: -1 }[event.key]
              if (next !== undefined) {
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

      <div role="tabpanel" id={`ma-panel-${tab}`} aria-labelledby={`ma-tab-${tab}`}>
        {tab === 'invite' ? <InviteForm members={members} /> : <Everyone members={members} />}
      </div>
    </main>
  )
}
