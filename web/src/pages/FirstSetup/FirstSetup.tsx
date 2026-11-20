import { useIsMutating, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, Navigate } from 'react-router'

import { onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { ApiError } from '../../api/client'
import { myContactsQuery, NotYoursError, RelinkedError } from '../../api/contacts'
import { type Person, personKeys, updateOwnProfile } from '../../api/persons'
import { type Tree, treeQuery, type TreePerson } from '../../api/tree'
import { ContactSwitch, contactSwitchKey } from '../../components/ContactSwitch/ContactSwitch'
import { byType, typeName } from '../../lib/contactTypes'
import { errorMessage, refusedMessage } from '../../lib/errorMessage'
import { familyLines } from '../../lib/familySides'
import { fullName, initials } from '../../lib/names'
import { useSignedIn } from '../../lib/useSignedIn'
import page from '../../styles/signInForm.module.css'
import styles from './FirstSetup.module.css'

const steps = 3

// design-notes §3.4: right after an invite's password is set, for the person the login is linked to.
export function FirstSetup() {
  const { user, person, personSettled, error } = useSignedIn()

  if (error instanceof ApiError && error.status === 401) {
    return <Navigate to="/auth/login" replace state={{ from: '/setup' }} />
  }
  if (user && !user.person_id) {
    return <Navigate to="/" replace />
  }

  return (
    <>
      <title>Getting started — Pluviose-Monestime Family Hub</title>
      <main className={`${page.main} ${page.tallMain}`}>
        <div className={`${page.form} ${styles.column}`}>
          {person ? (
            // Keyed by person: if an admin relinks the login, nothing typed for one person goes to another.
            <Steps key={person.id} person={person} />
          ) : error ? (
            <p role="alert">{errorMessage(error)}</p>
          ) : user && personSettled ? (
            <p role="alert">Your profile didn’t load. Refresh the page to try again.</p>
          ) : (
            <p className={page.hint}>Loading…</p>
          )}
        </div>
      </main>
    </>
  )
}

function Steps({ person }: { person: Person }) {
  const queryClient = useQueryClient()
  const [step, setStep] = useState(0)
  // Only what they've typed: until then it follows the story on file, which may be refreshed meanwhile.
  const [draft, setDraft] = useState<string | null>(null)
  const bio = draft ?? person.bio ?? ''
  const changed = draft !== null && draft.trim() !== (person.bio ?? '').trim()
  const heading = useRef<HTMLHeadingElement>(null)
  const moved = useRef(false)

  // The buttons stay put while the step above them changes, so focus moves to the new heading for screen readers.
  useEffect(() => {
    if (moved.current) {
      heading.current?.focus()
    }
  }, [step])

  // Held while a save runs: leaving its step would hide its failure, and with it a change that never saved.
  const switching = useIsMutating({ mutationKey: contactSwitchKey() }) > 0
  const goTo = (next: number) => {
    moved.current = true
    setStep(next)
  }
  const last = step === steps - 1

  const saveBio = useMutation({
    mutationFn: () => updateOwnProfile(person.id, { bio: bio.trim() || null }),
    onMutate: sessionGeneration,
    onSuccess: (updated, _, since) => {
      if (!sameSession(since)) {
        return
      }
      queryClient.setQueryData<Person>(personKeys.one(person.id), (old) => old && { ...old, bio: updated.bio })
      void queryClient.invalidateQueries({ queryKey: personKeys.one(person.id) })
      setDraft(null)
      goTo(1)
    },
    onError: onSessionEnded(queryClient),
  })

  const busy = saveBio.isPending || switching
  const onContinue = () => {
    if (busy) {
      return
    }
    if (step !== 0) {
      goTo(step + 1)
    } else {
      if (changed) {
        saveBio.mutate()
      } else {
        goTo(1)
      }
    }
  }

  return (
    <>
      <div className={styles.progress}>
        <span className={styles.stepCount}>
          Step {step + 1} of {steps}
        </span>
        <div aria-hidden="true" className={styles.bars}>
          {Array.from({ length: steps }, (_, i) => (
            <span key={i} className={i <= step ? `${styles.bar} ${styles.done}` : styles.bar} />
          ))}
        </div>
      </div>

      {step === 0 && (
        <section aria-labelledby="fs-h0" className={`${styles.step} ${styles.about}`}>
          <h1 ref={heading} id="fs-h0" tabIndex={-1} className={`${page.title} ${styles.title}`}>
            Let the family see who you are
          </h1>
          <div className={styles.photoRow}>
            <span aria-hidden="true" className={styles.avatar}>
              {initials(person)}
            </span>
            <div className={styles.photoActions}>
              {/* Photos arrive with uploads in Phase 5. */}
              <button type="button" aria-disabled="true" className={`pill ${styles.photoButton}`}>
                Add a photo
              </button>
              <span className={page.hint}>You can do this later too.</span>
            </div>
          </div>
          <div className={page.field}>
            <label htmlFor="fs-bio" className={page.label}>
              A line or two about you
            </label>
            <textarea
              id="fs-bio"
              rows={3}
              // Held still while it saves, so nothing typed meanwhile is lost.
              readOnly={saveBio.isPending}
              value={bio}
              onChange={(event) => {
                setDraft(event.target.value)
                saveBio.reset()
              }}
              className={`${page.input} ${styles.bio}`}
            />
          </div>
          {saveBio.isError && <p role="alert">{refusedMessage(saveBio.error)}</p>}
        </section>
      )}

      {step === 1 && (
        <section aria-labelledby="fs-h1" className={`${styles.step} ${styles.reach}`}>
          <h1 ref={heading} id="fs-h1" tabIndex={-1} className={`${page.title} ${styles.title}`}>
            How can family reach you?
          </h1>
          <p className={styles.text}>
            Phone, email, address and social accounts all start as “Only me.” Turn on what you’d like the family to see.
          </p>
          <Contacts personId={person.id} />
          <span className={page.hint}>Administrators can see all information you enter.</span>
        </section>
      )}

      {step === 2 && (
        <section aria-labelledby="fs-h2" className={`${styles.step} ${styles.place}`}>
          <h1 ref={heading} id="fs-h2" tabIndex={-1} className={`${page.title} ${styles.title}`}>
            Here’s your place in the family
          </h1>
          <Place person={person} />
        </section>
      )}

      <div className={styles.nav}>
        {step > 0 && (
          <button
            type="button"
            onClick={() => !busy && goTo(step - 1)}
            aria-disabled={busy || undefined}
            className={`pill ${page.secondary} ${styles.back}`}
          >
            Back
          </button>
        )}
        {last ? (
          <Link to="/" className={`pill ${page.primary} ${styles.next}`}>
            Go to my home page
          </Link>
        ) : (
          <button
            type="button"
            onClick={onContinue}
            aria-disabled={busy || undefined}
            className={`pill ${page.primary} ${styles.next}`}
          >
            Continue
          </button>
        )}
      </div>
      {!last && (
        <button
          type="button"
          onClick={() => {
            if (!busy) {
              saveBio.reset()
              goTo(step + 1)
            }
          }}
          aria-disabled={busy || undefined}
          className={styles.skip}
        >
          Skip for now
        </button>
      )}
    </>
  )
}

function Contacts({ personId }: { personId: string }) {
  const contacts = useQuery(myContactsQuery(personId))

  // A relink means the page is about to reload as the new person.
  if (contacts.isPending || contacts.error instanceof RelinkedError) {
    return <p className={page.hint}>Loading…</p>
  }
  // Rows read before a failed reload stay, as on My profile, unless they turned out to be someone else's.
  if (!contacts.data || contacts.error instanceof NotYoursError) {
    return <p role="alert">{errorMessage(contacts.error!)}</p>
  }
  if (contacts.data.length === 0) {
    return <p className={styles.text}>Nothing here yet. You can add a phone, email or address on My profile.</p>
  }
  return [...contacts.data].sort(byType).map((contact) => (
    <div key={contact.id} className={styles.contact}>
      <div className={styles.contactText}>
        <span id={`fs-${contact.id}`} className={styles.contactLabel}>
          {contact.label ?? typeName(contact)}
        </span>
        <span>{contact.value}</span>
      </div>
      <ContactSwitch personId={personId} contact={contact} labelledBy={`fs-${contact.id}`} size="small" />
    </div>
  ))
}

const listNames = (names: string[]) =>
  names.length <= 1 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`

function cousinsLine(cousins: TreePerson[]) {
  if (cousins.length === 0) {
    return ''
  }
  const shown = cousins.slice(0, 3).map((p) => p.first_name)
  const names = cousins.length > 3 ? `${shown.join(', ')} and ${cousins.length - 3} more` : listNames(shown)
  const sides = new Set(cousins.map((p) => p.side))
  const [side] = sides
  const where = sides.size === 1 && (side === 'first' || side === 'second') ? `on the ${familyLines[side]} side` : 'here too'
  return cousins.length === 1 ? `Your cousin ${names} is ${where}. ` : `Your cousins ${names} are ${where}. `
}

// Parents, grandparents and living first cousins (children of a parent's siblings), by birth or adoption: foster
// links aren't family (architecture §3.4).
function family(tree: Tree, personId: string) {
  const byId = new Map(tree.persons.map((p) => [p.id, p]))
  const parentIds = new Map<string, string[]>()
  const childIds = new Map<string, string[]>()
  for (const r of tree.relationships) {
    if (r.type === 'parent_child' && r.subtype !== 'foster') {
      parentIds.set(r.child_id, [...(parentIds.get(r.child_id) ?? []), r.parent_id])
      childIds.set(r.parent_id, [...(childIds.get(r.parent_id) ?? []), r.child_id])
    }
  }
  const people = (ids: Iterable<string>) => [...new Set(ids)].flatMap((id) => byId.get(id) ?? [])
  const parentsOf = (id: string) => parentIds.get(id) ?? []
  const childrenOf = (id: string) => childIds.get(id) ?? []

  const parents = parentsOf(personId)
  const grandparents = parents.flatMap(parentsOf)
  const siblings = new Set([personId, ...parents.flatMap(childrenOf)])
  const auntsAndUncles = new Set(grandparents.flatMap(childrenOf).filter((id) => !parents.includes(id)))
  const cousins = [...auntsAndUncles].flatMap(childrenOf).filter((id) => !siblings.has(id))
  return {
    parents: people(parents),
    grandparents: people(grandparents),
    cousins: people(cousins).filter((p) => !p.is_deceased),
  }
}

function Generation({ title, people, className }: { title: string; people: TreePerson[]; className?: string }) {
  if (people.length === 0) {
    return null
  }
  return (
    <>
      <span className={styles.generation}>{title}</span>
      <div className={styles.people}>
        {people.map((p) => (
          <span key={p.id} className={className}>
            {fullName(p)}
          </span>
        ))}
      </div>
      <span aria-hidden="true" className={styles.connector} />
    </>
  )
}

function Place({ person }: { person: Person }) {
  const tree = useQuery(treeQuery)
  const kin = useMemo(() => tree.data && family(tree.data, person.id), [tree.data, person.id])

  if (!kin) {
    return tree.isError ? <p role="alert">{errorMessage(tree.error)}</p> : <p className={page.hint}>Loading…</p>
  }
  return (
    <>
      <div className={styles.tree}>
        <Generation title="Grandparents" people={kin.grandparents} className={styles.grandparent} />
        <Generation title="Parents" people={kin.parents} className={styles.parent} />
        <span className={styles.you}>{person.first_name} · You</span>
      </div>
      <p className={styles.text}>
        {kin.parents.length === 0 && 'Your parents aren’t in the tree yet. '}
        {cousinsLine(kin.cousins)}Something look wrong? Let an admin know and they’ll fix it.
      </p>
    </>
  )
}
