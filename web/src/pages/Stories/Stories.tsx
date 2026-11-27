import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router'

import { draftStoriesQuery, publishedStoriesQuery, searchStoriesQuery, type Story, type StoryPerson } from '../../api/stories'
import { MenuIcon } from '../../components/MenuIcon/MenuIcon'
import { bornOn } from '../../lib/dates'
import { errorMessage } from '../../lib/errorMessage'
import { fullName, initials } from '../../lib/names'
import { useRelations } from '../../lib/useRelations'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './Stories.module.css'

// The Add a story guide's topics (design-notes §4.7b), in its order; any others follow A–Z.
const topicOrder = ['Growing up', 'Moving & immigration', 'Weddings', 'Work', 'Recipes', 'Holidays']
const tones = [styles.t1, styles.t2, styles.t3, styles.t4, styles.t5]
const stills = [styles.v1, styles.v2, styles.v3, styles.v4, styles.v5, styles.v6]
// The same person or story always gets the same colour.
const pick = <T,>(list: T[], id: string) => list[[...id].reduce((n, c) => (n * 31 + c.charCodeAt(0)) >>> 0, 0) % list.length]!
const stories = (n: number) => `${n} ${n === 1 ? 'story' : 'stories'}`
const recorded = (s: Story) => bornOn(s.recorded_on, s.recorded_on_precision)

// Stories (design-notes §4.6).
export function Stories() {
  const published = useQuery(publishedStoriesQuery)
  const drafts = useQuery(draftStoriesQuery)
  const [teller, setTeller] = useState<string | null>(null)
  const [topic, setTopic] = useState<string | null>(null)
  const [typed, setTyped] = useState('')
  const [q, setQ] = useState('')
  useEffect(() => {
    const wait = setTimeout(() => setQ(typed.trim()), 250)
    return () => clearTimeout(wait)
  }, [typed])
  const found = useQuery({ ...searchStoriesQuery(q), enabled: q !== '' })

  const all = useMemo(() => published.data ?? [], [published.data])
  const featured = all.find((s) => s.featured)
  const tellers = useMemo(() => {
    const byId = new Map<string, { person: StoryPerson; count: number }>()
    for (const s of all) {
      if (s.teller) {
        const t = byId.get(s.teller.id) ?? { person: s.teller, count: 0 }
        t.count++
        byId.set(s.teller.id, t)
      }
    }
    return [...byId.values()]
  }, [all])
  const topics = useMemo(() => {
    const seen = [...new Set(all.flatMap((s) => s.topics))]
    const rank = (t: string) => (topicOrder.includes(t) ? topicOrder.indexOf(t) : topicOrder.length)
    return seen.sort((a, b) => rank(a) - rank(b) || a.localeCompare(b))
  }, [all])
  const matched = q === '' ? all : found.data
  const shown = (matched ?? []).filter((s) => (!teller || s.teller?.id === teller) && (!topic || s.topics.includes(topic)))
  const tellerName = tellers.find((t) => t.person.id === teller)?.person
  const listTitle = tellerName ? `Stories told by ${fullName(tellerName)}` : (topic ?? 'All stories')
  const myDrafts = drafts.data ?? []

  return (
    <main className={styles.page}>
      <title>Stories — Pluviose Family Hub</title>
      <div className={styles.head}>
        <div className={styles.headText}>
          <h1 className={styles.title}>Family stories</h1>
          <p className={styles.sub}>
            {all.length > 0 ? `In their own words · ${stories(all.length)} from ${tellers.length} ${tellers.length === 1 ? 'storyteller' : 'storytellers'}` : 'In their own words'}
          </p>
        </div>
        {all.length > 0 && (
          <div className={styles.search}>
            <label htmlFor="story-search" className="visually-hidden">
              Search stories
            </label>
            <MenuIcon name="search" className={styles.searchIcon} />
            <input
              id="story-search"
              type="search"
              autoComplete="off"
              placeholder="Search stories and what was said"
              value={typed}
              onChange={(event) => setTyped(event.target.value)}
              onKeyDown={(event) => event.key === 'Escape' && setTyped('')}
              className={styles.searchInput}
            />
          </div>
        )}
      </div>
      {published.isPending ? (
        <p className={styles.muted}>Loading…</p>
      ) : published.isError ? (
        <p role="alert">{errorMessage(published.error)}</p>
      ) : (
        <>
          {featured && <Featured story={featured} />}
          {tellers.length > 0 && (
            <section aria-labelledby="storytellers">
              <h2 id="storytellers" className={styles.secLabel}>
                Storytellers
              </h2>
              <div className={styles.tellers}>
                {tellers.map(({ person, count }) => (
                  <button
                    key={person.id}
                    type="button"
                    aria-pressed={teller === person.id}
                    aria-label={`${fullName(person)}, ${stories(count)}`}
                    onClick={() => setTeller((was) => (was === person.id ? null : person.id))}
                    className={styles.teller}
                  >
                    <Portrait person={person} />
                    <b>{person.first_name}</b>
                    <small>{stories(count)}</small>
                  </button>
                ))}
              </div>
            </section>
          )}
          {drafts.isError && <p role="alert">Your drafts couldn’t be loaded. {errorMessage(drafts.error)}</p>}
          {myDrafts.length > 0 && (
            <section aria-labelledby="drafts" className={styles.album}>
              <h2 id="drafts" className={styles.secLabel}>
                Your drafts <small>only you and admins see these</small>
              </h2>
              <ul className={styles.prints}>
                {myDrafts.map((s) => (
                  <Print key={s.id} story={s} />
                ))}
              </ul>
            </section>
          )}
          {all.length === 0 ? (
            <div className={styles.empty}>
              <h2>No stories yet</h2>
              <p>Record a grandparent, an aunt, anyone with a story worth keeping, on a phone, in their own words. Add it here and the whole family can watch it.</p>
            </div>
          ) : (
            <section aria-labelledby="all-stories" className={styles.album}>
              <div className={styles.albumHead}>
                <h2 id="all-stories" className={styles.secLabel}>
                  {listTitle}
                </h2>
                {topics.length > 0 && (
                  <div role="group" aria-label="Topics" className={styles.chips}>
                    {topics.map((t) => (
                      <button key={t} type="button" aria-pressed={topic === t} onClick={() => setTopic((was) => (was === t ? null : t))} className={styles.chip}>
                        {t}
                      </button>
                    ))}
                  </div>
                )}
              </div>
              {q !== '' && found.isPending ? (
                <p className={styles.muted}>Searching…</p>
              ) : q !== '' && found.isError ? (
                <p role="alert">{errorMessage(found.error)}</p>
              ) : shown.length === 0 ? (
                <p className={styles.muted}>
                  {teller || topic ? 'No stories match. Try another storyteller or topic.' : `No stories match “${q}”.`}
                </p>
              ) : (
                <ul className={styles.prints}>
                  {shown.map((s) => (
                    <Print key={s.id} story={s} />
                  ))}
                </ul>
              )}
            </section>
          )}
        </>
      )}
    </main>
  )
}

function Featured({ story }: { story: Story }) {
  const { user } = useSignedIn()
  const { relations } = useRelations(user?.person_id ?? null, story.teller !== null)
  const relation = story.teller ? relations?.get(story.teller.id) : undefined
  const when = recorded(story)
  const about = [relation && (relation === 'You' ? relation : `Your ${relation.toLowerCase()}`), when && `recorded ${when}`]
    .filter(Boolean)
    .join(' · ')
  return (
    <section aria-label="Story of the week" className={styles.hero}>
      <Still story={story} />
      <div className={styles.heroText}>
        <span className={styles.kick}>Story of the week</span>
        <h2>{story.title}</h2>
        {story.teller && (
          <div className={styles.by}>
            <Portrait person={story.teller} small />
            <span>
              <b>Told by {fullName(story.teller)}</b>
              {about && <small>{about.charAt(0).toUpperCase() + about.slice(1)}</small>}
            </span>
          </div>
        )}
        {story.summary && <p>{story.summary}</p>}
        <Link to={`/stories/${story.id}`} className={styles.watch}>
          <PlayIcon />
          Watch the story
        </Link>
      </div>
    </section>
  )
}

function Print({ story }: { story: Story }) {
  const when = recorded(story)
  const who = [story.teller && fullName(story.teller), when].filter(Boolean).join(' · ')
  return (
    <li className={styles.print}>
      <Still story={story} />
      <h3>
        {/* Covers the whole print, so the print opens the story. */}
        <Link to={`/stories/${story.id}`} className={styles.open}>
          {story.title}
        </Link>
      </h3>
      {who && <p className={styles.who}>{who}</p>}
      {story.summary && <p className={styles.about}>{story.summary}</p>}
    </li>
  )
}

function Still({ story }: { story: Story }) {
  return (
    <span aria-hidden="true" className={`${styles.still} ${pick(stills, story.id)}`}>
      <span className={styles.play}>
        <i>
          <PlayIcon />
        </i>
      </span>
      {story.status === 'draft' && <span className={styles.draftChip}>Draft</span>}
    </span>
  )
}

function Portrait({ person, small = false }: { person: StoryPerson; small?: boolean }) {
  return (
    <span aria-hidden="true" className={[styles.portrait, small && styles.portraitSmall, pick(tones, person.id)].filter(Boolean).join(' ')}>
      {initials(person)}
    </span>
  )
}

function PlayIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M7 4.5v15l13-7.5z" />
    </svg>
  )
}
