import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { Link, useParams } from 'react-router'

import { onSessionEnded, sameSession, sessionGeneration } from '../../api/auth'
import { ApiError } from '../../api/client'
import { hideStory, publishedStoriesQuery, storyKeys, storyQuery, type Story } from '../../api/stories'
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog'
import { errorMessage } from '../../lib/errorMessage'
import { fullName } from '../../lib/names'
import { useRelations } from '../../lib/useRelations'
import { useSignedIn } from '../../lib/useSignedIn'
import { Portrait } from './Portrait'
import styles from './StoryPage.module.css'
import { recorded, stories, timestamp, yourRelation } from './storyText'

const spoken: Record<NonNullable<Story['language']>, string> = {
  ht: 'Spoken in Haitian Creole',
  fr: 'Spoken in French',
  en: 'Spoken in English',
  mixed: 'Spoken in more than one language',
}

// A story's own page (design-notes §4.7, layout "Theatre", owner's pick 2026-11-26). The player waits for uploads
// (Phase 5), so the transcript and chapters don't jump anywhere yet.
export function StoryPage() {
  const { id = '' } = useParams()
  const story = useQuery(storyQuery(id))
  const st = story.data
  const gone = story.error instanceof ApiError && story.error.status === 404

  return (
    <main className={styles.page}>
      <title>{`${st?.title ?? 'Story'} — Pluviose Family Hub`}</title>
      <Link to="/stories" className={styles.back}>
        ‹ Stories
      </Link>
      {story.isPending ? (
        <p className={styles.muted}>Loading…</p>
      ) : gone ? (
        <p className={styles.muted}>This story isn’t here. It may have been removed, or not shared yet.</p>
      ) : story.isError ? (
        <p role="alert">{errorMessage(story.error)}</p>
      ) : (
        // Keyed, so a search typed on one story doesn't carry over to the next.
        <Theatre key={st!.id} story={st!} />
      )}
    </main>
  )
}

function Theatre({ story }: { story: Story }) {
  const [q, setQ] = useState('')
  const lines = story.transcript ?? []
  const when = recorded(story)
  const where = [when && `Recorded ${when}`, story.place && (when ? `in ${story.place}` : `Recorded in ${story.place}`)].filter(Boolean).join(' ')
  return (
    <>
      <div className={styles.player}>
        <div className={styles.screen}>
          <p className={styles.coming}>
            <b>Video coming with uploads</b>
            The recording will play here, with captions.
          </p>
        </div>
      </div>
      <div className={styles.title}>
        <h1>{story.title}</h1>
        {where && <p>{where}</p>}
        {story.status === 'draft' && <p className={styles.draft}>Draft · not shared with the family yet</p>}
        <HideIt story={story} />
      </div>
      <div className={styles.cols}>
        <Transcript story={story} q={q} setQ={setQ} />
        <div className={styles.side}>
          {story.teller && <ToldBy story={story} />}
          {story.chapters && story.chapters.length > 0 && (
            <section aria-labelledby="chapters" className={styles.panel}>
              <h2 id="chapters">Chapters</h2>
              <ol className={styles.chapters}>
                {story.chapters.map((c, i) => (
                  <li key={i}>
                    {/* A heading to jump to only while the whole transcript shows. */}
                    {lines.length > 0 ? (
                      <a
                        href={`#chapter-${i}`}
                        onClick={(event) => {
                          if (q === '') {
                            return
                          }
                          // The search may have hidden the heading: bring it back before going to it.
                          event.preventDefault()
                          flushSync(() => setQ(''))
                          document.getElementById(`chapter-${i}`)?.scrollIntoView()
                        }}
                      >
                        <span>{timestamp(c.at)}</span>
                        {c.title}
                      </a>
                    ) : (
                      <p>
                        <span>{timestamp(c.at)}</span>
                        {c.title}
                      </p>
                    )}
                  </li>
                ))}
              </ol>
            </section>
          )}
          {(story.people.length > 0 || story.other_people.length > 0) && (
            <section aria-labelledby="people" className={styles.panel}>
              <h2 id="people">People in this story</h2>
              <ul className={styles.people}>
                {story.people.map((p) => (
                  <li key={p.id}>
                    <Link to={`/people/${p.id}`} className={styles.chip}>
                      {fullName(p)}
                    </Link>
                  </li>
                ))}
                {story.other_people.map((name) => (
                  <li key={name} className={`${styles.chip} ${styles.out}`}>
                    {name} (not in the tree)
                  </li>
                ))}
              </ul>
            </section>
          )}
          <MoreFrom story={story} />
        </div>
      </div>
    </>
  )
}

type Line = NonNullable<Story['transcript']>[number]

// The lines under their chapters, in one pass: the server sends both in order. Lines before the first chapter have none.
function byChapter(story: Story) {
  const chapters = story.chapters ?? []
  const groups: { chapter: number; lines: Line[] }[] = [{ chapter: -1, lines: [] }]
  let next = 0
  for (const line of story.transcript ?? []) {
    while (next < chapters.length && chapters[next]!.at <= line.at) {
      groups.push({ chapter: next++, lines: [] })
    }
    groups.at(-1)!.lines.push(line)
  }
  while (next < chapters.length) {
    groups.push({ chapter: next++, lines: [] })
  }
  return groups
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

function Transcript({ story, q, setQ }: { story: Story; q: string; setQ: (q: string) => void }) {
  const lines = story.transcript ?? []
  const wanted = q.trim()
  // One pattern both finds the lines and marks the words, so they never disagree.
  const pattern = wanted ? new RegExp(`(${escape(wanted)})`, 'giu') : null
  const groups = byChapter(story)
    .map((g) => ({ ...g, lines: pattern ? g.lines.filter((l) => l.text.search(pattern) >= 0) : g.lines }))
    .filter((g) => g.chapter >= 0 || g.lines.length > 0)
  const shown = pattern ? groups.filter((g) => g.lines.length > 0) : groups
  const chapters = story.chapters ?? []

  return (
    <section aria-labelledby="transcript" className={styles.panel}>
      <div className={styles.tHead}>
        <h2 id="transcript">Transcript</h2>
        {lines.length > 0 && (
          <input
            type="search"
            aria-label="Search the transcript"
            placeholder="Search what was said"
            value={q}
            onChange={(event) => setQ(event.target.value)}
            onKeyDown={(event) => event.key === 'Escape' && setQ('')}
            className={styles.tSearch}
          />
        )}
      </div>
      {story.language && <p className={styles.lang}>{spoken[story.language]}</p>}
      {lines.length === 0 ? (
        <p className={styles.muted}>No transcript yet.</p>
      ) : shown.length === 0 ? (
        <p className={styles.muted}>Nothing matches “{wanted}”.</p>
      ) : (
        <div className={styles.lines}>
          {shown.map((g) => (
            <div key={g.chapter}>
              {g.chapter >= 0 && (
                <h3 id={`chapter-${g.chapter}`} className={styles.chapterTitle}>
                  {chapters[g.chapter]!.title}
                </h3>
              )}
              {g.lines.map((line, j) => (
                <p key={j} className={styles.line}>
                  <span>{timestamp(line.at)}</span>
                  {pattern ? <Highlight text={line.text} pattern={pattern} /> : line.text}
                </p>
              ))}
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

function Highlight({ text, pattern }: { text: string; pattern: RegExp }) {
  return text.split(pattern).map((part, i) => (i % 2 === 1 ? <mark key={i}>{part}</mark> : part))
}

// Only the storyteller sees it, and only while the story is shared (§9.6).
function HideIt({ story }: { story: Story }) {
  const { user } = useSignedIn()
  const queryClient = useQueryClient()
  const [asking, setAsking] = useState(false)
  const button = useRef<HTMLButtonElement>(null)
  const hide = useMutation({
    mutationFn: () => hideStory(story.id),
    onMutate: sessionGeneration,
    onSuccess: (hidden, _, since) => {
      if (!sameSession(since)) {
        return
      }
      queryClient.setQueryData(storyKeys.one(story.id), hidden)
      void queryClient.invalidateQueries({ queryKey: storyKeys.all })
      setAsking(false)
    },
    onError: onSessionEnded(queryClient),
  })
  if (story.status !== 'published' || !story.teller || user?.person_id !== story.teller.id) {
    return null
  }
  return (
    <>
      <button ref={button} type="button" onClick={() => setAsking(true)} className={`pill ${styles.hide}`}>
        Hide this story
      </button>
      <ConfirmDialog
        open={asking}
        question={`Hide “${story.title}”?`}
        note="It goes back to a draft that only you, whoever added it and admins can see. It won’t be shared again unless you agree."
        yes="Hide it"
        no="Keep it shared"
        busy={hide.isPending}
        error={hide.error ? errorMessage(hide.error) : undefined}
        onYes={() => hide.mutate()}
        onCancel={() => {
          hide.reset()
          setAsking(false)
        }}
        focusAfterClose={() => button.current}
      />
    </>
  )
}

function ToldBy({ story }: { story: Story }) {
  const teller = story.teller!
  const { user } = useSignedIn()
  const { relations } = useRelations(user?.person_id ?? null)
  const all = useQuery(publishedStoriesQuery)
  const relation = relations?.get(teller.id)
  const count = all.data?.filter((s) => s.teller?.id === teller.id).length
  const about = [relation && yourRelation(relation), count && stories(count)].filter(Boolean).join(' · ')
  return (
    <section aria-label="Told by" className={styles.panel}>
      <span className={styles.kick}>Told by</span>
      <Link to={`/people/${teller.id}`} className={styles.teller}>
        <Portrait person={teller} />
        <span>
          <b>{fullName(teller)}</b>
          {about && <small>{about}</small>}
        </span>
      </Link>
    </section>
  )
}

function MoreFrom({ story }: { story: Story }) {
  const all = useQuery(publishedStoriesQuery)
  const teller = story.teller
  const more = teller ? (all.data ?? []).filter((s) => s.teller?.id === teller.id && s.id !== story.id) : []
  if (!teller || more.length === 0) {
    return null
  }
  return (
    <section aria-labelledby="more" className={styles.panel}>
      <h2 id="more">More from {teller.first_name}</h2>
      <ul className={styles.more}>
        {more.map((s) => (
          <li key={s.id}>
            <Link to={`/stories/${s.id}`}>
              <b>{s.title}</b>
              {recorded(s) && <small>{recorded(s)}</small>}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
