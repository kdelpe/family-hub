import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo } from 'react'
import { Link, useParams } from 'react-router'

import { ApiError } from '../../api/client'
import { askFor, type Contact, contactKeys, type ContactMethod, contactsQuery, isRequestable, type RequestKind } from '../../api/contacts'
import { getPerson, getPersons, type Person, personKeys } from '../../api/persons'
import { getTree, type Tree, treeKeys } from '../../api/tree'
import { birthday, lifespan, year } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { familyGroups } from '../../lib/familyGroups'
import { fullName, initials } from '../../lib/names'
import { gendered, relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './PersonProfile.module.css'

const kinds: RequestKind[] = ['phone', 'whatsapp', 'address']

const askWords: Record<RequestKind, { ask: string; asked: string }> = {
  phone: { ask: 'Request phone number', asked: '✓ Phone number requested' },
  whatsapp: { ask: 'Request WhatsApp', asked: '✓ WhatsApp requested' },
  address: { ask: 'Request address', asked: '✓ Address requested' },
}

// A full profile (design-notes §4.9). Photos and moments join it with media and events (Phase 5).
export function PersonProfile() {
  const { id = '' } = useParams()
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const person = useQuery({ queryKey: personKeys.one(id), queryFn: ({ signal }) => getPerson(id, { signal }) })
  const tree = useQuery({ queryKey: treeKeys.all, queryFn: ({ signal }) => getTree({ signal }) })
  const contacts = useQuery(contactsQuery(id))
  const relations = useMemo(
    () => (tree.data && viewerId ? relationsTo(tree.data, viewerId) : new Map<string, string>()),
    [tree.data, viewerId],
  )

  const notFound = person.error instanceof ApiError && person.error.status === 404
  if (person.error) {
    return (
      <main className={styles.page}>
        <title>{`${notFound ? 'Not found' : 'Profile'} — Pluviose-Monestime Family Hub`}</title>
        {notFound ? (
          <p>
            We couldn’t find that person. <Link to="/people">See everyone in People</Link>
          </p>
        ) : (
          <p role="alert">{errorMessage(person.error)}</p>
        )}
      </main>
    )
  }
  if (!person.data) {
    return null
  }
  const p = person.data
  const name = fullName(p)
  const relation = relations.get(p.id)
  const facts = [
    p.maiden_name && p.maiden_name !== p.last_name ? `née ${p.maiden_name}` : null,
    p.is_deceased ? lifespan(p) : birthday(p.birth_date) && `Birthday ${birthday(p.birth_date)}`,
    p.birth_place,
  ].filter(Boolean)

  return (
    <main className={styles.page}>
      <title>{`${name} — Pluviose-Monestime Family Hub`}</title>
      <Link to="/tree" className={styles.back}>
        ← Back to the family tree
      </Link>

      <section aria-labelledby="pp-name" className={styles.hero}>
        <div aria-hidden="true" className={styles.portrait}>
          {initials(p)}
        </div>
        <div className={styles.words}>
          {relation && <span className={styles.relation}>{relation}</span>}
          <h1 id="pp-name" className={styles.name}>
            {name}
          </h1>
          {facts.length > 0 && <span className={styles.facts}>{facts.join(' · ')}</span>}
          {p.bio && <p className={styles.bio}>{p.bio}</p>}
          <div className={styles.actions}>
            <Link to={`/tree?person=${encodeURIComponent(p.id)}`} className={`pill ${styles.center}`}>
              Center the tree on {p.first_name}
            </Link>
            {/* Only after a fetch that worked: a failed refresh mustn't keep showing a share taken back since. */}
            {contacts.isSuccess && (
              <Reach
                // A fresh button per person, so an ask on one profile doesn't show on the next.
                key={p.id}
                personId={p.id}
                contacts={contacts.data}
                // Admins and the person themself get every value; only what was shared with you shows here, and not
                // while a refresh (or its retries) runs, since the share may have been taken back.
                showShared={user?.role !== 'admin' && viewerId !== p.id && !contacts.isFetching}
              />
            )}
          </div>
          {contacts.error && <p role="alert">{errorMessage(contacts.error)}</p>}
        </div>
      </section>

      {tree.error && <p role="alert">{errorMessage(tree.error)}</p>}
      {tree.data && <Family tree={tree.data} person={p} relations={relations} />}
    </main>
  )
}

// The dashed request buttons, and any phone, WhatsApp or address shared with you (never asked-for values).
function Reach({ personId, contacts, showShared }: { personId: string; contacts: Contact[]; showShared: boolean }) {
  const shared = showShared
    ? contacts.filter((c): c is ContactMethod => !isRequestable(c) && c.visibility === 'on_request')
    : []
  return (
    <>
      {contacts
        .filter(isRequestable)
        .sort((a, b) => kinds.indexOf(a.kind) - kinds.indexOf(b.kind))
        .map((c) => (
          <AskButton key={c.kind} personId={personId} kind={c.kind} asked={c.my_request_status === 'requested'} />
        ))}
      {shared.map((c) => (
        <span key={c.id} className={styles.shared}>
          {c.label ?? (c.platform === 'whatsapp' ? 'WhatsApp' : c.kind === 'address' ? 'Address' : 'Phone')}: {c.value}
        </span>
      ))}
    </>
  )
}

function AskButton({ personId, kind, asked }: { personId: string; kind: RequestKind; asked: boolean }) {
  const queryClient = useQueryClient()
  const ask = useMutation({
    mutationFn: () => askFor(personId, kind),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: contactKeys.of(personId) }),
  })
  if (ask.data?.my_request_status === 'shared') {
    // Already shared: the refetched contacts show the value instead.
    return null
  }
  if (asked || ask.isSuccess) {
    return <span className={styles.asked}>{askWords[kind].asked}</span>
  }
  return (
    <span className={styles.ask}>
      <button
        type="button"
        className={`pill ${styles.askButton}`}
        aria-disabled={ask.isPending}
        onClick={() => {
          if (!ask.isPending) {
            ask.mutate()
          }
        }}
      >
        {askWords[kind].ask}
      </button>
      {ask.error && (
        <span role="alert" className={styles.askError}>
          {ask.error instanceof ApiError && ask.error.status === 403
            ? 'Your login isn’t linked to you in the tree yet, so you can’t ask. An admin can link it.'
            : errorMessage(ask.error)}
        </span>
      )}
    </span>
  )
}

function Family({ tree, person, relations }: { tree: Tree; person: Person; relations: Map<string, string> }) {
  const byId = new Map(tree.persons.map((t) => [t.id, t]))
  const g = familyGroups(tree, person.id)
  // The tree carries no dates (§10), so the relatives shown are fetched for theirs.
  const shown = [
    ...new Set([...g.parents, ...g.partners.map((x) => x.id), ...g.children, ...g.stepChildren.map((s) => s.id), ...g.grandchildren]),
  ].sort()
  const relatives = useQuery({
    queryKey: personKeys.some(shown),
    queryFn: ({ signal }) => getPersons(shown, { signal }),
    enabled: shown.length > 0,
  })
  const details = new Map(relatives.data?.map((r) => [r.id, r]))
  const via = new Map(g.stepChildren.map((s) => [s.id, s.via]))
  const born = (id: string) => year(details.get(id)?.birth_date ?? null, null) ?? '9999'
  const name = (id: string) => (byId.get(id) ? fullName(byId.get(id)!) : '')
  // Oldest first where years are known (they're masked for members), else by name; fathers before mothers.
  const byAge = (ids: string[]) => [...ids].sort((a, b) => born(a).localeCompare(born(b)) || name(a).localeCompare(name(b)))
  const parents = [...g.parents].sort((a, b) => Number(byId.get(b)?.gender === 'male') - Number(byId.get(a)?.gender === 'male'))

  // Who they are to you; for someone who has passed, their years; else their birthday.
  const note = (id: string) => {
    const d = details.get(id)
    if (d?.is_deceased) {
      return lifespan(d) ?? relations.get(id) ?? null
    }
    const b = d && birthday(d.birth_date, 'short')
    return relations.get(id) ?? (b ? `Birthday ${b}` : null)
  }
  const stepNote = (id: string, via: string) => {
    const child = byId.get(id)
    const parent = byId.get(via)
    return parent && child ? `${parent.first_name}’s ${gendered(child.gender, 'son', 'daughter', 'child')}` : null
  }
  const partnerTitle = (married: boolean, ended: boolean) =>
    married ? (ended ? 'Formerly married to' : 'Married to') : ended ? 'Former partner' : 'Partner'

  const groups = [
    { title: 'Parents', people: parents.map((id) => ({ id, note: note(id) })) },
    ...[true, false].flatMap((married) =>
      [false, true].map((ended) => ({
        title: partnerTitle(married, ended),
        people: g.partners.filter((x) => x.married === married && x.ended === ended).map((x) => ({ id: x.id, note: note(x.id) })),
      })),
    ),
    { title: 'Children', people: byAge(g.children).map((id) => ({ id, note: note(id) })) },
    {
      title: 'Step-children',
      people: byAge([...via.keys()]).map((id) => ({ id, note: stepNote(id, via.get(id)!) })),
    },
    { title: 'Grandchildren', people: byAge(g.grandchildren).map((id) => ({ id, note: note(id) })) },
  ].filter((group) => group.people.some((x) => byId.has(x.id)))

  if (groups.length === 0) {
    return null
  }
  return (
    <section aria-labelledby="pp-family" className={styles.family}>
      <h2 id="pp-family" className={styles.familyTitle}>
        {person.first_name}’s family
      </h2>
      {relatives.error && <p role="alert">{errorMessage(relatives.error)}</p>}
      {groups.map((group) => (
        <div key={group.title} className={styles.group}>
          <h3 className={styles.groupTitle}>{group.title}</h3>
          <ul className={styles.cards}>
            {group.people.map(({ id, note: text }) => {
              const relative = byId.get(id)
              return (
                relative && (
                  <li key={id}>
                    <Link to={`/people/${id}`} className={styles.card}>
                      <span aria-hidden="true" className={styles.avatar}>
                        {initials(relative)}
                      </span>
                      <span className={styles.cardWords}>
                        <span className={styles.cardName}>{fullName(relative)}</span>
                        {text && <span className={styles.cardNote}>{text}</span>}
                      </span>
                    </Link>
                  </li>
                )
              )
            })}
          </ul>
        </div>
      ))}
    </section>
  )
}
