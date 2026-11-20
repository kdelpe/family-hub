import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'

import {
  type ContactMethod,
  type DirectoryEntry,
  directoryQuery,
  isRequestable,
  kindOf,
  kindWords,
  platformLabel,
  type RequestKind,
  requestKinds,
} from '../../api/contacts'
import { treeQuery } from '../../api/tree'
import { AskButton } from '../../components/AskButton/AskButton'
import { CheckPill } from '../../components/CheckPill/CheckPill'
import { errorMessage } from '../../lib/errorMessage'
import { fullName, initials, nameMatches } from '../../lib/names'
import { relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './Directory.module.css'

const linkIcon = (
  <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24">
    <path d="M10 14 L14 10 M8.5 16.5 L6.8 18.2 A3.5 3.5 0 0 1 1.8 13.2 L5 10 A3.5 3.5 0 0 1 10 10 M15.5 7.5 L17.2 5.8 A3.5 3.5 0 0 1 22.2 10.8 L19 14 A3.5 3.5 0 0 1 14 14" />
  </svg>
)

const inSentence: Record<RequestKind, string> = { phone: 'phone number', whatsapp: 'WhatsApp', address: 'address' }

// "Phone number, WhatsApp and address"
const onRequestWords = (kinds: RequestKind[]) => {
  const words = kinds.map((k) => inSentence[k])
  const list = words.length > 1 ? `${words.slice(0, -1).join(', ')} and ${words.at(-1)}` : (words[0] ?? '')
  return list.charAt(0).toUpperCase() + list.slice(1)
}

const mailIcon = (
  <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24">
    <rect x="3" y="5.5" width="18" height="13" rx="2" />
    <path d="M3.5 7 L12 13 L20.5 7" />
  </svg>
)

// The local part may hold "?" or "&", which would add recipients or headers; "@" stays as is for mail apps.
const mailto = (email: string) => {
  const at = email.lastIndexOf('@')
  return `mailto:${encodeURIComponent(email.slice(0, at))}@${encodeURIComponent(email.slice(at + 1))}`
}

// The family phone book (design-notes §4.4): living relatives, with only what they chose to share.
export function Directory() {
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const directory = useQuery(directoryQuery)
  const tree = useQuery(treeQuery)
  const [query, setQuery] = useState('')
  const relations = useMemo(
    () => (tree.data && viewerId ? relationsTo(tree.data, viewerId) : new Map<string, string>()),
    [tree.data, viewerId],
  )
  const entries = useMemo(
    // Your own entry is managed in My profile, so it isn't listed (as in the mockup).
    // Only after a fetch that worked: a failed refresh mustn't keep showing what was hidden or taken back since.
    () => (directory.isSuccess ? directory.data : []).filter((e) => e.id !== viewerId && nameMatches(fullName(e), query)),
    [directory.isSuccess, directory.data, query, viewerId],
  )

  return (
    <main className={styles.page}>
      <title>Directory — Pluviose-Monestime Family Hub</title>
      <div className={styles.top}>
        <h1 className={styles.title}>Directory</h1>
        <label htmlFor="d-search" className="visually-hidden">
          Search the directory
        </label>
        <input
          id="d-search"
          type="search"
          placeholder="Search by name…"
          autoComplete="off"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          className={styles.search}
        />
      </div>

      {directory.error && <p role="alert">{errorMessage(directory.error)}</p>}
      {directory.isSuccess && entries.length === 0 && (
        <p className={styles.none}>{query.trim() ? 'Nobody here matches.' : 'Nobody else is in the directory yet.'}</p>
      )}
      {entries.length > 0 && (
        <ul className={styles.list}>
          {entries.map((e) => (
            <li key={e.id}>
              <Row entry={e} relation={relations.get(e.id) ?? null} />
            </li>
          ))}
        </ul>
      )}

      <p className={styles.footer}>
        Only living family members appear here, with just what they chose to share. When you ask for a phone number or
        address, they decide whether to share it with you. Manage your own in <Link to="/profile">My profile</Link>.
      </p>
    </main>
  )
}

function Row({ entry, relation }: { entry: DirectoryEntry; relation: string | null }) {
  const rows = entry.contacts.filter((c): c is ContactMethod => !isRequestable(c))
  const family = rows.filter((c) => c.visibility === 'family')
  const emails = family.filter((c) => c.kind === 'email')
  const socials = family.filter((c) => c.kind === 'social')
  const others = family.filter((c) => c.kind === 'other')
  // The values stay on the profile: the directory never lists them (design-notes §4.4).
  const sharedWithYou = requestKinds.filter((k) => rows.some((c) => c.shared_with_you && kindOf(c) === k))
  // An admin gets these rows in full and can't ask, so they only learn what's there to ask for.
  const onRequest = requestKinds.filter((k) =>
    rows.some((c) => c.visibility === 'on_request' && !c.shared_with_you && kindOf(c) === k),
  )
  const asks = entry.contacts.filter(isRequestable).sort((a, b) => requestKinds.indexOf(a.kind) - requestKinds.indexOf(b.kind))
  const name = fullName(entry)

  return (
    <article className={styles.row}>
      <Link to={`/people/${entry.id}`} className={styles.who}>
        <span aria-hidden="true" className={styles.avatar}>
          {initials(entry)}
        </span>
        <span className={styles.whoWords}>
          <span className={styles.name}>{name}</span>
          {relation && <span className={styles.relation}>{relation}</span>}
        </span>
      </Link>

      <div className={styles.reach}>
        {emails.map((c) => (
          <div key={c.id} className={styles.email}>
            <span className={styles.value}>{c.value}</span>
            <a href={mailto(c.value)} className={`pill tap ${styles.emailButton}`} aria-label={emails.length > 1 ? `Email ${entry.first_name} at ${c.value}` : `Email ${entry.first_name}`}>
              {mailIcon}
              Email
            </a>
          </div>
        ))}
        {socials.length > 0 && (
          <ul aria-label="Social accounts" className={styles.socials}>
            {socials.map((c) => {
              // The platform's name, so a label like "Old account" never hides where the link goes.
              const platformName = c.platform === 'other' ? (c.label ?? 'Other') : platformLabel(c.platform)
              // A value the server won't link (an "Other" account) still shows, as text.
              const label = c.href ? platformName : `${platformName}: ${c.value}`
              return (
                <li key={c.id}>
                  {c.href ? (
                    <a
                      href={c.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      aria-label={`${entry.first_name} on ${label}`}
                      className={`pill tap ${styles.social}`}
                    >
                      {linkIcon}
                      {label}
                    </a>
                  ) : (
                    <span className={styles.social}>
                      {linkIcon}
                      {label}
                    </span>
                  )}
                </li>
              )
            })}
          </ul>
        )}
        {others.map((c) => (
          <span key={c.id} className={styles.value}>
            {c.label ? `${c.label}: ${c.value}` : c.value}
          </span>
        ))}
        {sharedWithYou.length > 0 && (
          <div className={styles.asks}>
            {sharedWithYou.map((k) => (
              <CheckPill key={k} size="small" to={`/people/${entry.id}`}>
                {kindWords[k]} shared with you
              </CheckPill>
            ))}
          </div>
        )}
        {onRequest.length > 0 && (
          <span className={styles.onRequest}>
            {onRequestWords(onRequest)} on request
          </span>
        )}
        {asks.length > 0 && (
          <div className={styles.asks}>
            {asks.map((c) => (
              <AskButton
                key={c.kind}
                personId={entry.id}
                kind={c.kind}
                asked={c.my_request_status === 'requested'}
                size="small"
              />
            ))}
          </div>
        )}
      </div>
    </article>
  )
}
