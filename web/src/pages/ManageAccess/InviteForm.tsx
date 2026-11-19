import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useState } from 'react'
import { Link } from 'react-router'

import { adminKeys, invite, type InviteInput, type Member } from '../../api/admin'
import { ApiError } from '../../api/client'
import { getTree, type TreePerson, treeKeys } from '../../api/tree'
import { errorMessage } from '../../lib/errorMessage'
import { fullName, initials, nameMatches } from '../../lib/names'
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

export function InviteForm({ members }: { members: Member[] | undefined }) {
  const { person: me } = useSignedIn()
  const queryClient = useQueryClient()
  const tree = useQuery({ queryKey: treeKeys.all, queryFn: ({ signal }) => getTree({ signal }) })
  const [query, setQuery] = useState('')
  const [personId, setPersonId] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<Role>('member')
  // undefined until picked: the default is you, when you're in the tree.
  const [pickedFrom, setPickedFrom] = useState<string | null | undefined>(undefined)
  const [missingPerson, setMissingPerson] = useState(false)

  const senders = sendersFor(me, members)
  const fromId = pickedFrom !== undefined ? pickedFrom : (senders[0]?.personId ?? null)
  const from = senders.find((s) => s.personId === fromId) ?? null
  const invitee = tree.data?.persons.find((p) => p.id === personId) ?? null
  const withLogin = new Set(members?.flatMap((m) => (m.person ? [m.person.id] : [])))

  const send = useMutation({
    // The names ride along, so the note after sending says who it was for and from as they were when sent.
    mutationFn: ({ input }: { input: InviteInput; inviteeFirst: string; fromName: string | null }) => invite(input),
    onSuccess: () => {
      setQuery('')
      setPersonId(null)
      setEmail('')
      setRole('member')
      setPickedFrom(undefined)
      void queryClient.invalidateQueries({ queryKey: adminKeys.members })
    },
  })
  const sent = send.isSuccess ? send.variables : undefined

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
      input: { email, role, person_id: personId, ...(from ? { from_person_id: from.personId } : {}) },
      inviteeFirst: invitee?.first_name ?? '',
      fromName: from?.name ?? null,
    })
  }

  const matches =
    tree.data && query.trim() !== '' && !invitee
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
              send.reset()
            }}
            className={styles.input}
          />
          {tree.error && <p role="alert">{errorMessage(tree.error)}</p>}
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
          {tree.data && query.trim() !== '' && !invitee && matches.length === 0 && (
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
                  aria-pressed={s.personId === fromId}
                  className={
                    s.personId === fromId ? `${styles.choice} ${styles.sender} ${styles.choiceOn}` : `${styles.choice} ${styles.sender}`
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

        {send.error && (
          <p role="alert" className={styles.error}>
            {inviteErrorMessage(send.error, invitee)}
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
            <strong>From:</strong> Pluviose-Monestime Family Hub
          </span>
          <span>
            <strong>Subject:</strong> {from ? `${from.first} invited you to the family hub` : 'You’re invited to the family hub'}
          </span>
        </div>
        <div className={styles.mailBody}>
          <span className={styles.mailGreeting}>Bonjou{inviteeFirst ? ` ${inviteeFirst}` : ''},</span>
          <span>
            {from ? `${from.name} invited you to` : 'You’re invited to'} the Pluviose-Monestime Family Hub — a private place
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

// You first, then the other admins who can send one: linked to someone, signed up and not disabled.
function sendersFor(
  me: { id: string; first_name: string; last_name: string | null } | undefined,
  members: Member[] | undefined,
): Sender[] {
  const senders: Sender[] = []
  const add = (p: { id: string; first_name: string; last_name: string | null }, hint: string) => {
    if (!senders.some((s) => s.personId === p.id)) {
      senders.push({ personId: p.id, name: fullName(p), first: p.first_name, initials: initials(p), hint })
    }
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
      case 'validation_error':
        // The one refusal the form can't prevent besides a bad email: the sender lost their login meanwhile.
        return error.message.startsWith('from_person_id')
          ? 'That person can’t send invitations right now. Pick someone else.'
          : 'Check the email address and try again.'
    }
  }
  return errorMessage(error)
}
