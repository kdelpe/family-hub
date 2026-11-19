import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query'

import { adminKeys, type Member, sendLink } from '../../api/admin'
import { ApiError } from '../../api/client'
import { errorMessage } from '../../lib/errorMessage'
import { fullName } from '../../lib/names'
import styles from './ManageAccess.module.css'

const dayMs = 24 * 60 * 60 * 1000

export function memberStatus(member: Member, now: Date): string {
  switch (member.status) {
    case 'active':
      return 'Active'
    case 'disabled':
      return 'Disabled'
    case 'invited': {
      const left = member.invite_expires_at === null ? 0 : new Date(member.invite_expires_at).getTime() - now.getTime()
      if (left <= 0) {
        return 'Invited · expired'
      }
      const days = Math.ceil(left / dayMs)
      return `Invited · expires in ${days} ${days === 1 ? 'day' : 'days'}`
    }
  }
}

export function Everyone({ members, now = new Date() }: { members: UseQueryResult<Member[]>; now?: Date }) {
  if (members.error) {
    return <p role="alert">{errorMessage(members.error)}</p>
  }
  if (!members.data) {
    return null
  }
  return (
    <section aria-label="Everyone with access" className={styles.everyone}>
      <div className={styles.tableBox}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th scope="col">Name</th>
              <th scope="col">Sign-in email</th>
              <th scope="col">Role</th>
              <th scope="col">Status</th>
              <th scope="col">
                <span className="visually-hidden">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {members.data.map((member) => (
              <Row key={member.id} member={member} now={now} />
            ))}
          </tbody>
        </table>
      </div>
      <span className={styles.note}>Sign-in emails are only shown to admins.</span>
    </section>
  )
}

function Row({ member, now }: { member: Member; now: Date }) {
  const queryClient = useQueryClient()
  // Reread after, so a resent invite's status shows its new expiry.
  const send = useMutation({
    mutationFn: () => sendLink(member.id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: adminKeys.members }),
  })
  const action = member.status === 'active' ? 'Send password reset' : member.status === 'invited' ? 'Resend invitation' : null
  const name = member.person ? fullName(member.person) : 'Not linked to anyone yet'

  return (
    <tr>
      <td className={member.person ? styles.name : styles.unlinked}>{name}</td>
      <td className={styles.email}>{member.email}</td>
      <td>{member.role === 'admin' ? 'Admin' : 'Member'}</td>
      <td>
        <span className={member.status === 'active' ? `${styles.badge} ${styles.badgeActive}` : styles.badge}>
          {memberStatus(member, now)}
        </span>
      </td>
      <td className={styles.actionCell}>
        {action &&
          (send.isSuccess ? (
            <span role="status" className={styles.sent}>
              Email sent
            </span>
          ) : (
            <button
              type="button"
              className={`pill ${styles.rowButton}`}
              aria-disabled={send.isPending}
              aria-label={`${action} to ${name}`}
              onClick={() => {
                if (!send.isPending) {
                  send.mutate()
                }
              }}
            >
              {action}
            </button>
          ))}
        {send.error && (
          <span role="alert" className={styles.rowError}>
            {sendErrorMessage(send.error)}
          </span>
        )}
      </td>
    </tr>
  )
}

function sendErrorMessage(error: Error): string {
  if (error instanceof ApiError && error.code === 'mail_failed') {
    return 'The email couldn’t be sent. Try again.'
  }
  if (error instanceof ApiError && error.status === 404) {
    return 'This login is disabled or gone. Reload the page.'
  }
  return errorMessage(error)
}
