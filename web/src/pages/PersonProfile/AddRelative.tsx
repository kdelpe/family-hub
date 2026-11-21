import { useMutation, useQueryClient } from '@tanstack/react-query'
import { type KeyboardEvent, type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

import { ApiError } from '../../api/client'
import { type AddedRelative, addRelative, type NewRelative, type ParentKind, type Relation } from '../../api/relatives'
import { type Tree, treeKeys, type TreePerson } from '../../api/tree'
import { errorMessage } from '../../lib/errorMessage'
import type { FamilyLinks } from '../../lib/familyLinks'
import { fullName, initials, nameMatches } from '../../lib/names'
import { gendered } from '../../lib/relation'
import styles from './AddRelative.module.css'

type Step = 'who' | 'kind' | 'other' | 'married' | 'ended' | 'shared' | 'check'
type Who = { id: string } | { name: string }
type Named = { first_name: string; last_name: string | null }

const relationWord: Record<Relation, string> = { parent: 'parent', child: 'child', partner: 'partner', sibling: 'brother or sister' }

export const addLabel: Record<Relation, string> = {
  parent: 'Add a parent',
  child: 'Add a child',
  partner: 'Add a partner',
  sibling: 'Add a brother or sister',
}

type Answers = { kind: ParentKind; married: boolean; ended: boolean; siblings: 'full' | 'half' | 'unproven' }

// The relative's link word for the last screen and the toast: "daughter", "former wife", "adoptive father".
function linkWord(relation: Relation, gender: TreePerson['gender'], a: Answers) {
  switch (relation) {
    case 'partner':
      return (a.ended ? 'former ' : '') + (a.married ? gendered(gender, 'husband', 'wife', 'spouse') : 'partner')
    case 'sibling':
      // As relation.ts words it: the neutral word when the recorded parents prove neither full nor half (§3.4).
      return a.siblings === 'half'
        ? gendered(gender, 'half-brother', 'half-sister', 'half-sibling')
        : a.siblings === 'full'
          ? gendered(gender, 'brother', 'sister', 'sibling')
          : 'sibling'
    default: {
      const kind = a.kind === 'adoptive' ? 'adoptive ' : a.kind === 'foster' ? 'foster ' : ''
      return kind + (relation === 'parent' ? gendered(gender, 'father', 'mother', 'parent') : gendered(gender, 'son', 'daughter', 'child'))
    }
  }
}

// A refusal in the server's own words, without its "relationship conflict: " prefix.
function refusal(error: Error) {
  if (error instanceof ApiError && (error.status === 400 || error.status === 409)) {
    const reason = error.message.replace(/^(?:invalid relationship|invalid person|relationship conflict): /, '')
    return `${reason.charAt(0).toUpperCase()}${reason.slice(1)}.`
  }
  return errorMessage(error)
}

// "Ana Lucia Pluviose" → first name Ana, last name Lucia Pluviose; the profile can fix it afterwards.
const splitName = (name: string): Named => {
  const [first, ...rest] = name.split(' ')
  return { first_name: first ?? '', last_name: rest.join(' ') || null }
}

type Props = {
  tree: Tree
  byId: Map<string, TreePerson>
  links: FamilyLinks
  person: Named & { id: string }
  relation: Relation
  onRelation: (relation: Relation) => void
  onClose: () => void
  onAdded: (added: AddedRelative, isNew: boolean, word: string) => void
}

// One question per screen, and an answer moves on by itself (design-notes §4.9).
export function AddRelative({ tree, byId, links, person, relation, onRelation, onClose, onAdded }: Props) {
  const me = person.first_name
  // Fixed when the sheet opens, so a tree refetch midway can't add or drop a question under the admin.
  const [opened] = useState(() => {
    const parents = [...links.parentsOf(person.id)].filter((id) => byId.has(id))
    const partners = links.partnersOf(person.id).filter((p) => byId.has(p.id))
    return { parents, partners, people: new Map([...parents, ...partners.map((p) => p.id)].map((id) => [id, byId.get(id)!])) }
  })
  const { parents, partners } = opened
  // Their records as the sheet opened, in case a refetch drops one of them.
  const named = (id: string) => opened.people.get(id)!
  const [at, setAt] = useState(0)
  const [query, setQuery] = useState('')
  const [who, setWho] = useState<Who | null>(null)
  const [kind, setKind] = useState<ParentKind>('biological')
  const [other, setOther] = useState<string | null>(null)
  const [married, setMarried] = useState(true)
  const [ended, setEnded] = useState(false)
  const [shared, setShared] = useState<string | null>(null)
  const question = useRef<HTMLHeadingElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const queryClient = useQueryClient()

  const steps: Step[] = [
    'who',
    ...(relation === 'parent' || relation === 'child' ? (['kind'] as const) : []),
    ...(relation === 'child' && partners.length > 0 ? (['other'] as const) : []),
    ...(relation === 'partner' ? (['married', 'ended'] as const) : []),
    ...(relation === 'sibling' && parents.length > 1 ? (['shared'] as const) : []),
    'check',
  ]
  const step = steps[at]
  const relative = who && 'id' in who ? byId.get(who.id) : undefined
  const theirs: Named = relative ?? splitName(who && 'name' in who ? who.name : '')
  const theirName = fullName(theirs)
  const gender = relative?.gender ?? null
  // A sibling's parents once linked: the ones they already have, plus the ones linked now.
  const theirParents = relative ? links.parentsOf(relative.id) : new Set<string>()
  const via = shared ? [shared] : parents
  const after = new Set([...theirParents, ...via])
  const siblings =
    parents.some((p) => after.has(p)) && parents.some((p) => !after.has(p)) && [...after].some((p) => !parents.includes(p))
      ? 'half'
      : parents.length >= 2 && after.size === parents.length && parents.every((p) => after.has(p))
        ? 'full'
        : 'unproven'
  const word = linkWord(relation, gender, { kind, married, ended, siblings })
  const typed = query.trim().replace(/\s+/g, ' ')
  const matches = useMemo(() => {
    const found = []
    for (const p of typed ? tree.persons : []) {
      if (p.id !== person.id && nameMatches(fullName(p), typed) && found.push(p) === 5) {
        break
      }
    }
    return found
  }, [tree.persons, person.id, typed])

  const save = useMutation({
    mutationFn: (body: NewRelative) => addRelative(person.id, body),
    onSuccess: (added, body) => {
      void queryClient.invalidateQueries({ queryKey: treeKeys.all })
      onAdded(added, body.person !== undefined, word)
    },
  })

  // The page behind stays put and out of reach while the sheet is open.
  useEffect(() => {
    const root = document.getElementById('root')
    const overflow = document.body.style.overflow
    root?.setAttribute('inert', '')
    document.body.style.overflow = 'hidden'
    return () => {
      root?.removeAttribute('inert')
      document.body.style.overflow = overflow
    }
  }, [])

  useEffect(() => {
    if (step === 'who') {
      // With no parents recorded for a sibling, there's no search box to start in.
      const start = search.current ?? question.current
      start?.focus()
    } else {
      question.current?.focus()
    }
  }, [step])

  const go = (by: number) => {
    save.reset()
    setAt((n) => n + by)
  }
  const answer = (set: () => void) => () => {
    set()
    go(1)
  }

  const already = (id: string) => {
    switch (relation) {
      case 'parent':
        return links.parentsOf(person.id).has(id) || links.fosterParentsOf(person.id).has(id)
      case 'child':
        return links.childrenOf(person.id).includes(id) || links.fosterChildrenOf(person.id).includes(id)
      case 'partner':
        return partners.some((p) => p.id === id)
      case 'sibling':
        return parents.length > 0 && parents.every((p) => links.parentsOf(id).has(p))
    }
  }

  const submit = () => {
    const body: NewRelative = { relation }
    if (relative) {
      body.person_id = relative.id
    } else {
      body.person = theirs
    }
    if (relation === 'parent' || relation === 'child') {
      body.subtype = kind
    }
    if (relation === 'child' && other) {
      body.other_parent_id = other
    }
    if (relation === 'partner') {
      body.subtype = married ? 'married' : 'partner'
      body.ended = ended
    }
    if (relation === 'sibling' && shared) {
      body.shared_parent_id = shared
    }
    save.mutate(body)
  }

  // Not while saving: the answer, a refusal included, needs the sheet to show on.
  const dismiss = () => {
    if (!save.isPending) {
      onClose()
    }
  }
  // Escape in a filled search box clears it, as the browser's own clear would; otherwise it closes.
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Escape') {
      return
    }
    e.preventDefault()
    if (e.target === search.current && query) {
      setQuery('')
    } else {
      dismiss()
    }
  }

  const firstOf = (id: string) => (opened.people.get(id) ?? byId.get(id))?.first_name ?? ''
  const avatar = (p: Named | undefined) =>
    p && (
      <span aria-hidden="true" className={styles.avatar}>
        {initials(p)}
      </span>
    )
  const choice = (key: string, onPick: () => void, title: string, sub?: string | null, lead?: ReactNode, disabled = false) => (
    <li key={key}>
      <button type="button" className={styles.option} onClick={onPick} disabled={disabled}>
        {lead}
        <span className={styles.optionWords}>
          <span className={styles.optionTitle}>{title}</span>
          {sub && <span className={styles.optionSub}>{sub}</span>}
        </span>
      </button>
    </li>
  )

  let heading: string
  let screen: ReactNode
  if (step === 'who' && relation === 'sibling' && parents.length === 0) {
    heading = `Who are ${me}’s parents?`
    screen = (
      <>
        <p className={styles.problem}>
          Brothers and sisters are linked through the parents they share, and {me} has none recorded yet. Add a parent first.
        </p>
        <ul className={styles.options}>{choice('parent', () => onRelation('parent'), `Add a parent for ${me}`)}</ul>
      </>
    )
  } else if (step === 'who') {
    heading = `Who is ${me}’s ${relationWord[relation]}?`
    screen = (
      <>
        <p className={styles.hint}>Search the tree, or type the full name of someone new.</p>
        <input
          ref={search}
          type="search"
          className={styles.search}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Type a name"
          aria-label="Name"
          autoComplete="off"
        />
        <ul className={styles.options}>
          {matches.map((p) => {
            const parentNames = [...links.parentsOf(p.id)].map(firstOf).filter(Boolean)
            const taken = already(p.id)
            const sub = taken
              ? `Already ${me}’s ${relationWord[relation]}`
              : parentNames.length > 0
                ? `Child of ${parentNames.join(' and ')}`
                : null
            return choice(p.id, answer(() => setWho({ id: p.id })), fullName(p), sub, avatar(p), taken)
          })}
          {typed && (
            <li>
              <button type="button" className={`${styles.option} ${styles.newOption}`} onClick={answer(() => setWho({ name: typed }))}>
                <span aria-hidden="true" className={styles.plus}>
                  +
                </span>
                <span className={styles.optionWords}>
                  <span className={styles.optionTitle}>Add “{typed}” as someone new</span>
                  <span className={styles.optionSub}>Birthday, photo and story can be added on their profile</span>
                </span>
              </button>
            </li>
          )}
        </ul>
      </>
    )
  } else if (step === 'kind') {
    heading = `How is ${theirs.first_name} ${me}’s ${relation}?`
    const fostered = 'Shown on the tree, not counted as family'
    const options: [ParentKind, string, string | null][] =
      relation === 'parent'
        ? [
            ['biological', `Birth ${gendered(gender, 'father', 'mother', 'parent')}`, null],
            ['adoptive', `Adopted ${me}`, null],
            ['foster', `Fostered ${me}`, fostered],
          ]
        : [
            ['biological', `Born to ${me}`, null],
            ['adoptive', `Adopted by ${me}`, null],
            ['foster', `Fostered by ${me}`, fostered],
          ]
    screen = <ul className={styles.options}>{options.map(([k, title, sub]) => choice(k, answer(() => setKind(k)), title, sub))}</ul>
  } else if (step === 'other') {
    heading = `Who is ${theirs.first_name}’s other parent?`
    screen = (
      <ul className={styles.options}>
        {partners.map((p) => {
          const word = linkWord('partner', named(p.id).gender, { kind, married: p.married, ended: p.ended, siblings })
          return choice(p.id, answer(() => setOther(p.id)), fullName(named(p.id)), `${me}’s ${word}`, avatar(named(p.id)))
        })}
        {choice('none', answer(() => setOther(null)), 'Someone else, or not known yet', 'You can add them later')}
      </ul>
    )
  } else if (step === 'married') {
    heading = `Are ${theirs.first_name} and ${me} married?`
    screen = (
      <ul className={styles.options}>
        {choice('yes', answer(() => setMarried(true)), 'Yes, married')}
        {choice('no', answer(() => setMarried(false)), 'No, partners')}
      </ul>
    )
  } else if (step === 'ended') {
    heading = 'Are they still together?'
    screen = (
      <ul className={styles.options}>
        {choice('together', answer(() => setEnded(false)), 'Yes, still together')}
        {choice('ended', answer(() => setEnded(true)), 'No, it ended', 'Divorced, separated or widowed')}
      </ul>
    )
  } else if (step === 'shared') {
    heading = `Which parents do ${theirs.first_name} and ${me} share?`
    const sibling = gendered(gender, 'brother', 'sister', 'sibling')
    // "Both" only for two: the server links a full sibling through every parent, and a third would be one too many.
    screen = (
      <ul className={styles.options}>
        {parents.length === 2 &&
          choice('all', answer(() => setShared(null)), `Both: ${parents.map(firstOf).join(' and ')}`, `Full ${sibling}`)}
        {parents.map((p) =>
          theirParents.has(p)
            ? choice(p, () => {}, `Only ${fullName(named(p))}`, `Already ${theirs.first_name}’s parent`, avatar(named(p)), true)
            : choice(p, answer(() => setShared(p)), `Only ${fullName(named(p))}`, null, avatar(named(p))),
        )}
      </ul>
    )
  } else {
    heading = `Add ${theirName}?`
    const linking = relation === 'sibling' ? via.filter((p) => !theirParents.has(p)) : []
    const also =
      relation === 'child' && other
        ? `${firstOf(other)} is the other parent.`
        : linking.length > 0
          ? `They’ll be linked as a child of ${linking.map(firstOf).join(' and ')}.`
          : ''
    screen = (
      <>
        <div className={styles.preview}>
          <div className={styles.pair}>
            <span className={styles.mini}>
              {avatar(person)}
              {fullName(person)}
            </span>
            <span className={styles.link}>{word}</span>
            <span className={`${styles.mini} ${styles.miniNew}`}>
              {avatar(theirs)}
              {theirName}
            </span>
          </div>
          <p className={styles.sentence}>
            {theirName} will be {me}’s {word}. {also}
          </p>
        </div>
        {save.error && (
          <p role="alert" className={styles.problem}>
            {refusal(save.error)}
          </p>
        )}
        <button type="button" className={styles.primary} onClick={submit} disabled={save.isPending}>
          {save.isPending ? 'Adding…' : `Add ${theirs.first_name}`}
        </button>
      </>
    )
  }

  return createPortal(
    <div
      className={styles.scrim}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) {
          dismiss()
        }
      }}
    >
      <div className={styles.sheet} role="dialog" aria-modal="true" aria-labelledby="add-relative-question" onKeyDown={onKeyDown}>
        <div className={styles.bar}>
          <button type="button" className={styles.nav} onClick={() => go(-1)} hidden={at === 0} disabled={save.isPending}>
            ← Back
          </button>
          <span className={styles.dots} role="img" aria-label={`Step ${at + 1} of ${steps.length}`}>
            {steps.map((s, i) => (
              <span key={s} className={i <= at ? `${styles.dot} ${styles.dotOn}` : styles.dot} />
            ))}
          </span>
          <button type="button" className={`${styles.nav} ${styles.cancel}`} onClick={dismiss}>
            Cancel
          </button>
        </div>
        <div key={step} className={styles.screen}>
          <h2 id="add-relative-question" ref={question} tabIndex={-1} className={styles.question}>
            {heading}
          </h2>
          {screen}
        </div>
      </div>
    </div>,
    document.body,
  )
}
