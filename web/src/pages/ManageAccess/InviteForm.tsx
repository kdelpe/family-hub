import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { type FormEvent, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'

import { type AccessRequestItem, adminKeys, invite, type InviteInput, type Member } from '../../api/admin'
import { ApiError } from '../../api/client'
import { type TreePerson, treeQuery } from '../../api/tree'
import { errorMessage } from '../../lib/errorMessage'
import { foldName, fullName, initials, nameMatches } from '../../lib/names'
import { placeInTree } from '../../lib/placeInTree'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './ManageAccess.module.css'

type Role = 'member' | 'admin'
type Sender = { personId: string; name: string; first: string; initials: string; hint: string }

const roles: { id: Role; label: string; hint: string }[] = [
  { id: 'member', label: 'Member', hint: 'Sees the family, edits their own profile' },
  { id: 'admin', label: 'Admin', hint: 'Can also edit the tree and invite people' },
]

const maxMatches = 6

// A request being approved; n changes on every click, so approving the same one again fills the form again.
export type Approval = { request: AccessRequestItem; n: number }

export function InviteForm({ members, approval }: { members: UseQueryResult<Member[]>; approval: Approval | null }) {
  const { user, person } = useSignedIn()
  // The members list names you too, for when your own person couldn't be read.
  const me = person ?? members.data?.find((m) => m.id === user?.id)?.person ?? undefined
  const queryClient = useQueryClient()
  const tree = useQuery(treeQuery)
  const [query, setQuery] = useState('')
  const [personId, setPersonId] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('member')
  // null until picked: the default is you, when you're in the tree.
  const [pickedFrom, setPickedFrom] = useState<string | null>(null)
  const [missingPerson, setMissingPerson] = useState(false)
  const [request, setRequest] = useState<AccessRequestItem | null>(null)
  const [seenApproval, setSeenApproval] = useState(approval)
  const [pickName, setPickName] = useState<string | null>(null)
  // Each approval starts a new form; a send from an earlier one doesn't clear it or show its outcome here.
  const [generation, setGeneration] = useState(0)
  const generationNow = useRef(generation)
  useEffect(() => {
    generationNow.current = generation
  }, [generation])

  const withLogin = new Set(members.data?.flatMap((m) => (m.person ? [m.person.id] : [])))
  if (approval !== seenApproval) {
    setSeenApproval(approval)
    if (approval) {
      const r = approval.request
      setGeneration((g) => g + 1)
      setRequest(r)
      setQuery(r.full_name)
      setPersonId(null)
      setPickName(r.full_name)
      setEmail(r.email)
      setRole('member')
      setPickedFrom(r.suggested_by?.person_id ?? null)
      setMissingPerson(false)
    }
  }
  // Picked once the tree and logins are known: when exactly one living person without a login has that name.
  if (pickName !== null && tree.data && members.data) {
    setPickName(null)
    const same = tree.data.persons.filter(
      (p) => !p.is_deceased && !withLogin.has(p.id) && foldName(fullName(p)) === foldName(pickName),
    )
    if (same.length === 1) {
      setPersonId(same[0]!.id)
    }
  }
  // The request is approved only while the email is still theirs, so an unrelated invite can't approve it.
  const approving = request && email.trim().toLowerCase() === request.email.toLowerCase() ? request : null

  const senders = sendersFor(me, members.data, approving)
  // A pick that is no longer a sender (disabled meanwhile) falls back to you, so what's pressed is what's sent.
  const from = senders.find((s) => s.personId === pickedFrom) ?? senders.find((s) => s.personId === me?.id) ?? null
  const invitee = tree.data?.persons.find((p) => p.id === personId) ?? null

  const send = useMutation({
    // The names ride along, so the note after sending says who it was for and from as they were when sent.
    mutationFn: ({ input }: { input: InviteInput; inviteeFirst: string; fromName: string | null; generation: number }) =>
      invite(input),
    onSuccess: (_, sent) => {
      void queryClient.invalidateQueries({ queryKey: adminKeys.members })
      void queryClient.invalidateQueries({ queryKey: adminKeys.requests })
      if (sent.generation !== generationNow.current) {
        return
      }
      setQuery('')
      setPersonId(null)
      setEmail('')
      setRole('member')
      setPickedFrom(null)
      setRequest(null)
    },
  })
  const current = send.variables?.generation === generation
  const sent = send.isSuccess && current ? send.variables : undefined
  const sendError = current ? send.error : null

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (send.isPending) {
      return
    }
    if (!personId) {
      setMissingPerson(true)
      return
    }
    send.mutate({
      input: {
        email,
        role,
        person_id: personId,
        ...(from ? { from_person_id: from.personId } : {}),
        ...(approving ? { access_request_id: approving.id } : {}),
      },
      inviteeFirst: invitee?.first_name ?? '',
      fromName: from?.name ?? null,
      generation,
    })
  }

  // Waits for the members too, so someone who already has a login is never offered.
  const matches =
    tree.data && members.data && query.trim() !== '' && !invitee
      ? tree.data.persons
          .filter((p) => !p.is_deceased && nameMatches(fullName(p), query))
          .sort((a, b) => fullName(a).localeCompare(fullName(b)))
          .slice(0, maxMatches)
      : []

  return (
    <section aria-label="Invite someone" className={styles.invite}>
      <form className={styles.inviteForm} onSubmit={onSubmit}>
        {sent && <SentNote sent={sent} />}

        <div className={styles.step}>
          <h2 className={styles.stepTitle}>1. Who are you inviting?</h2>
          <span className={styles.stepHint}>Find them in the tree first so we don’t add them twice.</span>
          <label htmlFor="ma-find" className="visually-hidden">
            Find a person in the tree
          </label>
          <input
            id="ma-find"
            type="search"
            autoComplete="off"
            value={invitee ? fullName(invitee) : query}
            onChange={(event) => {
              setQuery(event.target.value)
              setPersonId(null)
              setPickName(null)
              if (!send.isPending) {
                send.reset()
              }
            }}
            className={styles.input}
          />
          {(tree.error ?? members.error) && <p role="alert">{errorMessage((tree.error ?? members.error)!)}</p>}
          {invitee && tree.data && (
            <div className={`${styles.personCard} ${styles.personCardOn}`}>
              <Avatar person={invitee} />
              <span className={styles.personWords}>
                <span className={styles.strong}>{fullName(invitee)}</span>
                <span className={styles.small}>{placeInTree(tree.data, invitee.id)}</span>
              </span>
              <span className={styles.selected}>Selected</span>
            </div>
          )}
          {matches.length > 0 && (
            <ul aria-label="People in the tree" className={styles.matches}>
              {matches.map((p) => {
                const taken = withLogin.has(p.id)
                return (
                  <li key={p.id}>
                    <button
                      type="button"
                      className={styles.personCard}
                      aria-disabled={taken}
                      onClick={() => {
                        if (taken) {
                          return
                        }
                        setPersonId(p.id)
                        setMissingPerson(false)
                      }}
                    >
                      <Avatar person={p} />
                      <span className={styles.personWords}>
                        <span className={styles.strong}>{fullName(p)}</span>
                        <span className={styles.small}>
                          {taken ? 'Already has a login' : placeInTree(tree.data!, p.id)}
                        </span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
          {tree.data && members.data && query.trim() !== '' && !invitee && matches.length === 0 && (
            <span className={styles.small}>Nobody in the tree by that name.</span>
          )}
          {missingPerson && !invitee && (
            <span role="alert" className={styles.error}>
              Pick who you’re inviting first.
            </span>
          )}
          <Link to="/tree" className={styles.addFirst}>
            Not in the tree yet? Add them first
          </Link>
        </div>

        <div className={styles.step}>
          <h2 className={styles.stepTitle}>2. Their email and role</h2>
          <label htmlFor="ma-email" className={styles.label}>
            Email
          </label>
          <input
            id="ma-email"
            type="email"
            required
            autoComplete="off"
            value={email}
            onChange={(event) => {
              setEmail(event.target.value)
              if (send.isError) {
                send.reset()
              }
            }}
            className={styles.input}
          />
          <div role="group" aria-label="Role" className={styles.roles}>
            {roles.map((r) => (
              <button
                key={r.id}
                type="button"
                aria-pressed={role === r.id}
                className={role === r.id ? `${styles.choice} ${styles.choiceOn}` : styles.choice}
                onClick={() => setRole(r.id)}
              >
                <span className={styles.strong}>{r.label}</span>
                <span className={styles.hint}>{r.hint}</span>
              </button>
            ))}
          </div>
        </div>

        <div className={styles.step}>
          <h2 className={styles.stepTitle}>3. Who is the invitation from?</h2>
          <span className={styles.stepHint}>People are more likely to join when they see a name they know.</span>
          {senders.length === 0 ? (
            <span className={styles.small}>Link your login to yourself in the tree to send it in your name.</span>
          ) : (
            <div role="group" aria-label="Invitation from" className={styles.senders}>
              {senders.map((s) => (
                <button
                  key={s.personId}
                  type="button"
                  aria-pressed={s.personId === from?.personId}
                  className={
                    s.personId === from?.personId ? `${styles.choice} ${styles.sender} ${styles.choiceOn}` : `${styles.choice} ${styles.sender}`
                  }
                  onClick={() => setPickedFrom(s.personId)}
                >
                  <span className={styles.senderAvatar}>{s.initials}</span>
                  <span className={styles.personWords}>
                    <span className={styles.strong}>{s.name}</span>
                    <span className={styles.hint}>{s.hint}</span>
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>

        {sendError && (
          <p role="alert" className={styles.error}>
            {inviteErrorMessage(sendError, invitee)}
          </p>
        )}
        <button type="submit" className={`pill ${styles.send}`} aria-disabled={send.isPending}>
          Send invitation
        </button>
      </form>

      <Preview inviteeFirst={invitee?.first_name ?? null} from={from} approver={from && from.personId !== me?.id ? me : null} />
    </section>
  )
}

function SentNote({ sent }: { sent: { inviteeFirst: string; fromName: string | null } }) {
  return (
    <div role="status" className={styles.sentNote}>
      <strong>
        Invitation sent to {sent.inviteeFirst}
        {sent.fromName ? `, from ${sent.fromName}` : ''}
      </strong>
      <span className={styles.sentHint}>It works for 7 days. You can resend it from the Everyone tab.</span>
    </div>
  )
}

function Preview({
  inviteeFirst,
  from,
  approver,
}: {
  inviteeFirst: string | null
  from: Sender | null
  approver: { first_name: string; last_name: string | null } | null | undefined
}) {
  return (
    <aside aria-label="Invitation preview" className={styles.preview}>
      <h2 className={styles.stepTitle}>4. What they’ll receive</h2>
      <div className={styles.mail}>
        <div className={styles.mailHead}>
          <span>
            <strong>From:</strong> Pluviose Family Hub
          </span>
          <span>
            <strong>Subject:</strong> {from ? `${from.first} invited you to the family hub` : 'You’re invited to the family hub'}
          </span>
        </div>
        <div className={styles.mailBody}>
          <span className={styles.mailGreeting}>Bonjou{inviteeFirst ? ` ${inviteeFirst}` : ''},</span>
          <span>
            {from ? `${from.name} invited you to` : 'You’re invited to'} the Pluviose Family Hub — a private place
            for our family’s tree, photos and stories.
          </span>
          {approver && <span className={styles.small}>Approved by {fullName(approver)}, admin.</span>}
          <span className={styles.mailButton}>Set up my account</span>
          <span className={styles.small}>
            Set a password to get started. The link works for 7 days. Administrators can see all information you enter.
          </span>
        </div>
      </div>
    </aside>
  )
}

function Avatar({ person }: { person: { first_name: string; last_name: string | null } }) {
  return <span className={styles.avatar}>{initials(person)}</span>
}

// Whoever suggested the person being approved, then you, then the other admins: each linked to someone, signed
// up and not disabled, as the server requires of a sender.
function sendersFor(
  me: { id: string; first_name: string; last_name: string | null } | undefined,
  members: Member[] | undefined,
  approving: AccessRequestItem | null,
): Sender[] {
  const senders: Sender[] = []
  const add = (p: { id: string; first_name: string; last_name: string | null }, hint: string) => {
    if (!senders.some((s) => s.personId === p.id)) {
      senders.push({ personId: p.id, name: fullName(p), first: p.first_name, initials: initials(p), hint })
    }
  }
  const suggester = members?.find(
    (m) => m.status === 'active' && m.person && m.person.id === approving?.suggested_by?.person_id,
  )?.person
  if (suggester && approving && suggester.id !== me?.id) {
    add(suggester, `Suggested ${approving.full_name.split(' ')[0]}`)
  }
  if (me) {
    add(me, 'You')
  }
  for (const m of members ?? []) {
    if (m.role === 'admin' && m.status === 'active' && m.person) {
      add(m.person, 'Admin')
    }
  }
  return senders
}

function inviteErrorMessage(error: Error, invitee: TreePerson | null): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'email_taken':
        return 'That email already has a login.'
      case 'person_taken':
        return `${invitee?.first_name ?? 'They'} already has a login.`
      case 'mail_failed':
        return 'The email couldn’t be sent, so nothing was saved. Try again.'
      case 'conflict':
        return 'Another admin already handled this request. Reload the page.'
      case 'validation_error':
        // Besides a bad email, the person or the sender may have changed since the page loaded.
        return 'Check the email address. If it looks right, reload the page: who you picked may have changed.'
    }
  }
  return errorMessage(error)
}
