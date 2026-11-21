import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router'

import { ApiError } from '../../api/client'
import { type Contact, type ContactMethod, contactsQuery, isRequestable, requestKinds } from '../../api/contacts'
import { getPerson, getPersons, type Person, personKeys } from '../../api/persons'
import { type AddedRelative, type Relation, undoRelative } from '../../api/relatives'
import { type Tree, treeKeys, treeQuery } from '../../api/tree'
import { AskButton, SharedValue } from '../../components/AskButton/AskButton'
import { birthday, lifespan, year } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { familyGroups } from '../../lib/familyGroups'
import { familyLinks, type FamilyLinks } from '../../lib/familyLinks'
import { fullName, initials } from '../../lib/names'
import { gendered, relationsTo } from '../../lib/relation'
import { useSignedIn } from '../../lib/useSignedIn'
import { AddRelative, addLabel } from './AddRelative'
import styles from './PersonProfile.module.css'

// A full profile (design-notes §4.9). Photos and moments join it with media and events (Phase 5).
export function PersonProfile() {
  const { id = '' } = useParams()
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const isAdmin = user?.role === 'admin'
  // Up here, not in Family: Undo stays while the admin opens the new person's profile to check it.
  const [added, setAdded] = useState<Added | null>(null)
  const queryClient = useQueryClient()
  const undo = useMutation({
    mutationFn: ({ added, isNew }: Added) => undoRelative(added, isNew),
    // Every person read too: the profile on screen may be the one just taken off.
    onSettled: () => Promise.all([treeKeys.all, ['persons']].map((queryKey) => queryClient.invalidateQueries({ queryKey }))),
    // Only that add's toast: another add may have come in while this Undo ran.
    onSuccess: (_, undone) => setAdded((current) => (current === undone ? null : current)),
  })
  const person = useQuery({ queryKey: personKeys.one(id), queryFn: ({ signal }) => getPerson(id, { signal }) })
  const tree = useQuery(treeQuery)
  const contacts = useQuery(contactsQuery(id))
  const links = useMemo(() => tree.data && familyLinks(tree.data), [tree.data])
  const relations = useMemo(
    () => (tree.data && links && viewerId ? relationsTo(tree.data, viewerId, links) : new Map<string, string>()),
    [tree.data, links, viewerId],
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
                // Not while a refresh (or its retries) runs, since the share may have been taken back.
                showShared={!contacts.isFetching}
              />
            )}
          </div>
          {contacts.error && <p role="alert">{errorMessage(contacts.error)}</p>}
        </div>
      </section>

      {tree.error && <p role="alert">{errorMessage(tree.error)}</p>}
      {tree.data && links && (
        <Family
          // A fresh section per person, so a sheet or an Undo for one profile never shows on the next.
          key={p.id}
          tree={tree.data}
          links={links}
          person={p}
          relations={relations}
          isAdmin={isAdmin}
          freshId={added?.added.person.id}
          onAdded={(next) => {
            undo.reset()
            setAdded(next)
          }}
        />
      )}
      {added && (
        // No time limit: Undo stays until it's used or put away.
        <div role="status" className={styles.toast}>
          <span>{undo.error ? errorMessage(undo.error) : added.text}</span>
          <span className={styles.toastActions}>
            <button type="button" onClick={() => undo.mutate(added)} disabled={undo.isPending}>
              {undo.isPending ? 'Undoing…' : 'Undo'}
            </button>
            <button type="button" onClick={() => setAdded(null)}>
              Done
            </button>
          </span>
        </div>
      )}
    </main>
  )
}

// The dashed request buttons, and any phone, WhatsApp or address shared with you (never asked-for values).
function Reach({ personId, contacts, showShared }: { personId: string; contacts: Contact[]; showShared: boolean }) {
  const shared = showShared ? contacts.filter((c): c is ContactMethod => !isRequestable(c) && c.shared_with_you) : []
  return (
    <>
      {contacts
        .filter(isRequestable)
        .sort((a, b) => requestKinds.indexOf(a.kind) - requestKinds.indexOf(b.kind))
        .map((c) => (
          <AskButton
            key={c.kind}
            personId={personId}
            kind={c.kind}
            asked={c.my_request_status === 'requested'}
            size="large"
          />
        ))}
      {shared.map((c) => (
        <SharedValue key={c.id} contact={c} />
      ))}
    </>
  )
}

type Added = { added: AddedRelative; isNew: boolean; text: string }

function Family({
  tree,
  links,
  person,
  relations,
  isAdmin,
  freshId,
  onAdded,
}: {
  tree: Tree
  links: FamilyLinks
  person: Person
  relations: Map<string, string>
  isAdmin: boolean
  freshId: string | undefined
  onAdded: (added: Added) => void
}) {
  const byId = new Map(tree.persons.map((t) => [t.id, t]))
  const g = familyGroups(tree, person.id, links)
  const [adding, setAdding] = useState<Relation | null>(null)
  const opener = useRef<HTMLButtonElement | null>(null)
  // After the sheet's cleanup has lifted inert from the page, or the focus call would be ignored.
  useEffect(() => {
    if (!adding) {
      opener.current?.focus()
    }
  }, [adding])
  // The tree carries no dates (§10), so the relatives shown are fetched for theirs.
  const shown = [
    ...new Set([
      ...g.parents,
      ...g.fosterParents,
      ...g.partners.map((x) => x.id),
      ...g.siblings,
      ...g.children,
      ...g.fosterChildren,
      ...g.stepChildren.map((s) => s.id),
      ...g.grandchildren,
    ]),
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
  // Foster links aren't family (§3.4), but an admin who adds one should see it land.
  const foster = (id: string, word: string) => ({ id, note: relations.get(id) ?? word })

  type Group = { title: string; people: { id: string; note: string | null }[]; add?: Relation }
  const partnerGroups: Group[] = [true, false]
    .flatMap((married) =>
      [false, true].map((ended) => ({
        title: partnerTitle(married, ended),
        people: g.partners.filter((x) => x.married === married && x.ended === ended).map((x) => ({ id: x.id, note: note(x.id) })),
      })),
    )
    .filter((group) => group.people.some((x) => byId.has(x.id)))
  // An admin adds a partner from the last partner group, or from an empty one.
  const partnerAdd: Group[] =
    partnerGroups.length > 0
      ? partnerGroups.map((group, i) => (i === partnerGroups.length - 1 ? { ...group, add: 'partner' } : group))
      : [{ title: 'Partners', people: [], add: 'partner' }]

  const groups = [
    {
      title: 'Parents',
      people: [...parents.map((id) => ({ id, note: note(id) })), ...g.fosterParents.map((id) => foster(id, 'Foster parent'))],
      add: 'parent' as const,
    },
    ...partnerAdd,
    { title: 'Siblings', people: byAge(g.siblings).map((id) => ({ id, note: note(id) })), add: 'sibling' as const },
    {
      title: 'Children',
      people: [...byAge(g.children).map((id) => ({ id, note: note(id) })), ...g.fosterChildren.map((id) => foster(id, 'Foster child'))],
      add: 'child' as const,
    },
    {
      title: 'Step-children',
      people: byAge([...via.keys()]).map((id) => ({ id, note: stepNote(id, via.get(id)!) })),
    },
    { title: 'Grandchildren', people: byAge(g.grandchildren).map((id) => ({ id, note: note(id) })) },
  ].filter((group: Group) => group.people.some((x) => byId.has(x.id)) || (isAdmin && group.add))

  if (groups.length === 0) {
    return null
  }
  const close = () => setAdding(null)
  return (
    <section aria-labelledby="pp-family" className={styles.family}>
      <h2 id="pp-family" className={styles.familyTitle}>
        {person.first_name}’s family
      </h2>
      {relatives.error && <p role="alert">{errorMessage(relatives.error)}</p>}
      {groups.map((group: Group) => (
        // Keyed by its Add card where it has one, so the card (and focus on it) survives a partner group renaming.
        <div key={group.add ?? group.title} className={styles.group}>
          <h3 className={styles.groupTitle}>{group.title}</h3>
          <ul className={styles.cards}>
            {group.people.map(({ id, note: text }) => {
              const relative = byId.get(id)
              return (
                relative && (
                  <li key={id}>
                    <Link to={`/people/${id}`} className={id === freshId ? `${styles.card} ${styles.fresh}` : styles.card}>
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
            {isAdmin && group.add && (
              <li>
                <button
                  type="button"
                  className={styles.addCard}
                  onClick={(e) => {
                    opener.current = e.currentTarget
                    setAdding(group.add!)
                  }}
                >
                  <span aria-hidden="true" className={styles.plus}>
                    +
                  </span>
                  {addLabel[group.add]}
                </button>
              </li>
            )}
          </ul>
        </div>
      ))}
      {adding && (
        <AddRelative
          key={adding}
          tree={tree}
          byId={byId}
          links={links}
          person={person}
          relation={adding}
          onRelation={setAdding}
          onClose={close}
          onAdded={(relative, isNew, word) => {
            close()
            onAdded({ added: relative, isNew, text: `${relative.person.first_name} added as ${person.first_name}’s ${word}.` })
          }}
        />
      )}
    </section>
  )
}
