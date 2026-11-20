import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query'

import { type AccessRequestItem, adminKeys, declineRequest, reopenRequest } from '../../api/admin'
import { ApiError } from '../../api/client'
import { errorMessage } from '../../lib/errorMessage'
import { sideLabel } from '../../lib/familySides'
import styles from './ManageAccess.module.css'

const dayMs = 24 * 60 * 60 * 1000

// "Requested 2 days ago", counted in calendar days on the viewer's clock.
export function requestedWhen(request: AccessRequestItem, now: Date): string {
  const verb = request.source === 'member_suggestion' ? 'Suggested' : 'Requested'
  const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const days = Math.max(0, Math.round((startOfDay(now) - startOfDay(new Date(request.created_at))) / dayMs))
  if (days === 0) {
    return `${verb} today`
  }
  return days === 1 ? `${verb} yesterday` : `${verb} ${days} days ago`
}

// "Daughter of Frantz Monestime": the relationship they gave, to each name they gave.
export function relatedLine(relationship: string | null, name: string): string {
  if (!relationship) {
    return name
  }
  return `${relationship[0]!.toUpperCase()}${relationship.slice(1)} of ${name}`
}

export function Requests({
  requests,
  onApprove,
  now = new Date(),
}: {
  requests: UseQueryResult<AccessRequestItem[]>
  onApprove: (request: AccessRequestItem) => void
  now?: Date
}) {
  if (requests.error) {
    return <p role="alert">{errorMessage(requests.error)}</p>
  }
  if (!requests.data) {
    return null
  }
  return (
    <section aria-label="Access requests" className={styles.requests}>
      {requests.data.length === 0 && <p className={styles.muted}>No requests waiting.</p>}
      {requests.data.map((request) => (
        <Request key={request.id} request={request} onApprove={onApprove} now={now} />
      ))}
      <p className={styles.note}>
        Requests come from the “Request access” page and from members who suggest a relative from their profile.
      </p>
    </section>
  )
}

function Request({
  request,
  onApprove,
  now,
}: {
  request: AccessRequestItem
  onApprove: (request: AccessRequestItem) => void
  now: Date
}) {
  const queryClient = useQueryClient()
  const change = useMutation({
    mutationFn: () => (request.status === 'pending' ? declineRequest(request.id) : reopenRequest(request.id)),
    onSettled: () => queryClient.invalidateQueries({ queryKey: adminKeys.requests }),
  })
  const act = () => {
    if (!change.isPending) {
      change.mutate()
    }
  }

  return (
    <article aria-labelledby={`ar-${request.id}`} className={styles.request}>
      <div className={styles.requestWords}>
        <div className={styles.requestHead}>
          <h2 id={`ar-${request.id}`} className={styles.stepTitle}>
            {request.full_name}
          </h2>
          <span className={styles.small}>{request.email}</span>
        </div>
        <div className={styles.chips}>
          {request.side && <span className={styles.chip}>{sideLabel(request.side)}</span>}
          <span className={`${styles.chip} ${styles.chipSource}`}>
            {request.source === 'self'
              ? 'Asked on the sign-in page'
              : `Suggested by ${request.suggested_by?.name ?? 'a member'}`}
          </span>
        </div>
        {request.related_names.length > 0 && (
          <div className={styles.related}>
            <strong className={styles.relatedTitle}>Related to</strong>
            {request.related_names.map((related, index) => (
              <span key={index} className={styles.relatedLine}>
                {relatedLine(request.relationship, related.name)}
                <span className={related.match ? styles.found : `${styles.found} ${styles.notFound}`}>
                  {related.match ? '✓ In the tree' : 'Not found — check spelling'}
                </span>
              </span>
            ))}
          </div>
        )}
        {request.note && <p className={styles.requestNote}>“{request.note}”</p>}
        <span className={styles.small}>{requestedWhen(request, now)}</span>
      </div>

      <div className={styles.requestActions}>
        {request.status === 'pending' ? (
          <>
            <button
              type="button"
              className={`pill ${styles.approve}`}
              aria-disabled={change.isPending}
              onClick={() => {
                if (!change.isPending) {
                  onApprove(request)
                }
              }}
            >
              Approve &amp; invite
            </button>
            <button type="button" className={`pill ${styles.notNow}`} aria-disabled={change.isPending} onClick={act}>
              Not now
            </button>
            <span className={styles.actionHint}>
              Approving takes you to the invite form so you can place them in the tree first.
            </span>
          </>
        ) : (
          <div className={styles.setAside}>
            <span className={styles.strong}>Set aside</span>
            <span className={styles.small}>They won’t be told. You can undo this for 90 days.</span>
            <button type="button" className={styles.undo} aria-disabled={change.isPending} onClick={act}>
              Undo
            </button>
          </div>
        )}
        {change.error && (
          <span role="alert" className={styles.rowError}>
            {changeErrorMessage(change.error, request.status)}
          </span>
        )}
      </div>
    </article>
  )
}

function changeErrorMessage(error: Error, status: AccessRequestItem['status']): string {
  // An undo is refused both when someone already undid it and when a newer request from that email is waiting.
  if (error instanceof ApiError && status === 'declined' && error.status === 409) {
    return 'This request changed meanwhile, or a newer one from this email is waiting. Reload the page.'
  }
  if (error instanceof ApiError && (error.status === 404 || error.status === 409)) {
    return 'This request changed meanwhile. Reload the page.'
  }
  return errorMessage(error)
}
