import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, m } from 'motion/react'
import { useEffect, useMemo, useRef, useState } from 'react'

import { createEvent, createEventKey, type DatePrecision, eventKeys, type NewEvent as NewEventBody } from '../../api/events'
import { treeQuery } from '../../api/tree'
import { errorMessage, refusedMessage } from '../../lib/errorMessage'
import { fullName, nameMatches } from '../../lib/names'
import styles from './Events.module.css'

const precisions: { id: DatePrecision; label: string }[] = [
  { id: 'day', label: 'Exact day' },
  { id: 'month', label: 'Month' },
  { id: 'year', label: 'Year' },
  { id: 'circa', label: 'About' },
]

const savedHold = 700

const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

// dayUnfinished: the date picker holds part of a date, which it reports as an empty value.
type Draft = { precision: DatePrecision; day: string; dayUnfinished: boolean; month: string; year: string }

// The date as the API takes it, null when none was given, or 'invalid' when what's typed isn't one yet.
function dateOf({ precision, day, dayUnfinished, month, year }: Draft): string | null | 'invalid' {
  if (precision === 'day') {
    return dayUnfinished ? 'invalid' : day || null
  }
  if (year === '' && (precision !== 'month' || month === '')) {
    return null
  }
  if (!/^\d{4}$/.test(year) || year === '0000') {
    return 'invalid'
  }
  if (precision === 'month') {
    return month ? `${year}-${month}` : 'invalid'
  }
  return year
}

// The + New event panel (design-notes §4.5): admins only, inline above the timeline.
export function NewEvent({ onClose, onSaved }: { onClose: () => void; onSaved: (title: string) => void }) {
  const queryClient = useQueryClient()
  const tree = useQuery(treeQuery)
  const [title, setTitle] = useState('')
  const [place, setPlace] = useState('')
  const [draft, setDraft] = useState<Draft>({ precision: 'day', day: '', dayUnfinished: false, month: '', year: '' })
  const [story, setStory] = useState('')
  const [people, setPeople] = useState<{ id: string; name: string }[]>([])
  const [adding, setAdding] = useState(false)
  const [query, setQuery] = useState('')
  const first = useRef<HTMLInputElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const addButton = useRef<HTMLButtonElement>(null)

  useEffect(() => first.current?.focus(), [])
  useEffect(() => {
    if (adding) {
      search.current?.focus()
    }
  }, [adding])

  const typed = query.trim().replace(/\s+/g, ' ')
  const matches = useMemo(() => {
    const found = []
    for (const p of typed ? (tree.data?.persons ?? []) : []) {
      if (!people.some((x) => x.id === p.id) && nameMatches(fullName(p), typed) && found.push(p) === 5) {
        break
      }
    }
    return found
  }, [tree.data, people, typed])

  const [done, setDone] = useState(false)
  // Closed during the hold some other way, it no longer reports the save as closing it.
  const hold = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(hold.current), [])
  const save = useMutation({
    mutationKey: createEventKey,
    mutationFn: (body: NewEventBody) => createEvent(body),
    onSuccess: (saved) => {
      void queryClient.invalidateQueries({ queryKey: eventKeys.all })
      // The check on Save shows for a moment before the panel closes.
      setDone(true)
      hold.current = setTimeout(() => onSaved(saved.title), savedHold)
    },
  })

  const date = dateOf(draft)
  const canSave = title.trim() !== '' && date !== 'invalid' && !save.isPending && !done

  const submit = () => {
    if (!canSave) {
      return
    }
    save.mutate({
      title: title.trim(),
      ...(date && { date, date_precision: draft.precision }),
      ...(place.trim() && { location: place.trim() }),
      ...(story.trim() && { description: story.trim() }),
      person_ids: people.map((p) => p.id),
    })
  }

  // The chip's own × goes away with it, so focus stays in the picker.
  const unpick = (id: string) => {
    setPeople((list) => list.filter((x) => x.id !== id))
    requestAnimationFrame(() => (search.current ?? addButton.current)?.focus())
  }

  const pick = (id: string, name: string) => {
    setPeople((list) => [...list, { id, name }])
    setQuery('')
    setAdding(false)
    // The search box goes away, so focus goes back to where another can be added.
    requestAnimationFrame(() => addButton.current?.focus())
  }

  return (
    <section aria-labelledby="ev-new" aria-busy={save.isPending} className={styles.panel}>
      {/* Locked while saving: an edit made now would be lost when the saved panel closes. */}
      <fieldset disabled={save.isPending || done} className={styles.locked}>
        <div className={styles.panelHead}>
          <h2 id="ev-new" className={styles.panelTitle}>
            New event
          </h2>
          <button type="button" onClick={onClose} aria-label="Close" className={`pill ${styles.close}`}>
            ×
          </button>
        </div>
        <div className={styles.pair}>
          <div className={styles.field}>
            <label htmlFor="ev-title" className={styles.label}>
              What happened?
            </label>
            <input
              ref={first}
              id="ev-title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g. Rose’s 90th birthday"
              maxLength={200}
              className={styles.input}
            />
          </div>
          <div className={styles.field}>
            <label htmlFor="ev-place" className={styles.label}>
              Where?
            </label>
            <input id="ev-place" type="text" value={place} onChange={(e) => setPlace(e.target.value)} maxLength={200} className={styles.input} />
          </div>
        </div>
        <fieldset className={styles.when}>
          <legend className={styles.label}>When?</legend>
          <div className={styles.whenRow}>
            <div role="group" aria-label="How exact is the date?" className={styles.switch}>
              {precisions.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  aria-pressed={draft.precision === p.id}
                  // Switching away drops a half-typed day the picker never reported, so its flag goes too.
                onClick={() => setDraft((d) => ({ ...d, precision: p.id, dayUnfinished: d.precision === p.id && d.dayUnfinished }))}
                  className={styles.switchOption}
                >
                  {draft.precision === p.id && (
                    <m.span layoutId="event-precision" className={styles.switchPill} transition={{ type: 'spring', stiffness: 420, damping: 34 }} />
                  )}
                  <span className={styles.switchLabel}>{p.label}</span>
                </button>
              ))}
            </div>
            {draft.precision === 'day' ? (
              <input
                type="date"
                aria-label="Date"
                value={draft.day}
                onChange={(e) => setDraft((d) => ({ ...d, day: e.target.value, dayUnfinished: e.target.validity.badInput }))}
              // A half-typed date fires no change event, only input.
              onInput={(e) => {
                const { value, validity } = e.currentTarget
                setDraft((d) => ({ ...d, day: value, dayUnfinished: validity.badInput }))
              }}
              aria-invalid={date === 'invalid'}
              aria-describedby={date === 'invalid' ? 'ev-date-hint' : undefined}
                className={`${styles.input} ${styles.dateInput}`}
              />
            ) : (
              <>
                {draft.precision === 'month' && (
                  <select
                    aria-label="Month"
                    value={draft.month}
                    onChange={(e) => setDraft((d) => ({ ...d, month: e.target.value }))}
                    className={`${styles.input} ${styles.monthInput}`}
                  >
                    <option value="">Month</option>
                    {months.map((m, i) => (
                      <option key={m} value={String(i + 1).padStart(2, '0')}>
                        {m}
                      </option>
                    ))}
                  </select>
                )}
                <input
                  type="text"
                  inputMode="numeric"
                  aria-label={draft.precision === 'circa' ? 'About which year' : 'Year'}
                  placeholder={draft.precision === 'circa' ? 'e.g. 1965' : 'Year'}
                  maxLength={4}
                  value={draft.year}
                  onChange={(e) => setDraft((d) => ({ ...d, year: e.target.value.replace(/\D/g, '') }))}
                  aria-invalid={date === 'invalid'}
                  aria-describedby={date === 'invalid' ? 'ev-date-hint' : undefined}
                  className={`${styles.input} ${styles.yearInput}`}
                />
              </>
            )}
          </div>
          {date === 'invalid' && (
            <span id="ev-date-hint" className={styles.hint}>
              {draft.precision === 'day'
              ? 'Finish the date, or clear it.'
              : draft.precision === 'month' && draft.month === ''
                ? 'Pick the month too.'
                : 'Write the year with four digits, like 1965.'}
            </span>
          )}
      </fieldset>
      <div className={styles.field}>
        <label htmlFor="ev-text" className={styles.label}>
          Tell the story <span className={styles.optional}>(optional)</span>
        </label>
        <textarea id="ev-text" rows={2} value={story} onChange={(e) => setStory(e.target.value)} maxLength={5000} className={styles.textarea} />
      </div>
      <div className={styles.who}>
        <span id="ev-who" className={styles.label}>
          Who was there?
        </span>
        <ul aria-labelledby="ev-who" className={styles.chips}>
          {people.map((p) => (
            <li key={p.id} className={styles.chosen}>
              {p.name}
              <button
                type="button"
                aria-label={`Remove ${p.name}`}
                onClick={() => unpick(p.id)}
                className={styles.remove}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
        {!adding && (
          <button ref={addButton} type="button" onClick={() => setAdding(true)} className={`pill ${styles.addPerson}`}>
            + Add person
          </button>
        )}
      </div>
      {adding && (
        <div className={styles.find}>
          <input
            ref={search}
            type="search"
            aria-label="Find a relative"
            placeholder="Type a name"
            autoComplete="off"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.preventDefault()
                setQuery('')
                setAdding(false)
                requestAnimationFrame(() => addButton.current?.focus())
              }
            }}
            className={styles.input}
          />
          {typed && (
            <ul className={styles.matches}>
              {matches.map((p) => (
                <li key={p.id}>
                  <button type="button" onClick={() => pick(p.id, fullName(p))} className={styles.match}>
                    {fullName(p)}
                  </button>
                </li>
              ))}
              {matches.length === 0 && tree.data && <li className={styles.muted}>No one by that name.</li>}
              {tree.isPending && <li className={styles.muted}>Loading…</li>}
              {tree.isError && !tree.data && (
                <li role="alert" className={styles.muted}>
                  Can’t search the family tree right now. {errorMessage(tree.error)}
                </li>
              )}
            </ul>
          )}
        </div>
      )}
      {save.isError && <p role="alert">{refusedMessage(save.error)}</p>}
      <div className={styles.actions}>
        <button type="button" onClick={onClose} className={`pill ${styles.cancel}`}>
          Cancel
        </button>
        <button type="button" onClick={submit} disabled={!canSave} className={`pill ${styles.save} ${done ? styles.saveDone : ''}`}>
          <AnimatePresence mode="wait" initial={false}>
            {done ? (
              <m.span key="done" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className={styles.saveFace}>
                <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
                  <m.path
                    d="M5 12l5 5 9-10"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="3"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    initial={{ pathLength: 0 }}
                    animate={{ pathLength: 1 }}
                    transition={{ duration: 0.35 }}
                  />
                </svg>
                Saved
              </m.span>
            ) : save.isPending ? (
              <m.span key="saving" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className={styles.saveFace}>
                <span className="visually-hidden">Saving…</span>
                {[0, 1, 2].map((i) => (
                  <m.i
                    key={i}
                    aria-hidden="true"
                    className={styles.saveDot}
                    animate={{ y: [0, -5, 0] }}
                    transition={{ duration: 0.6, repeat: Infinity, delay: i * 0.12 }}
                  />
                ))}
              </m.span>
            ) : (
              <m.span key="idle" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -6 }} className={styles.saveFace}>
                Save event
              </m.span>
            )}
          </AnimatePresence>
        </button>
      </div>
      </fieldset>
    </section>
  )
}
