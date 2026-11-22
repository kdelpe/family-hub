import { AnimatePresence, m } from 'motion/react'
import { type KeyboardEvent, type PointerEvent, useRef } from 'react'

import styles from './Events.module.css'

const dragPxPerYear = 26

export function YearHandle({ years, current, onPick }: { years: string[]; current: string; onPick: (year: string, glide: boolean) => void }) {
  const at = Math.max(0, years.indexOf(current))
  const drag = useRef<{ id: number; y0: number; from: number } | null>(null)
  const step = (by: number) => {
    const next = Math.min(years.length - 1, Math.max(0, at + by))
    if (next !== at) {
      onPick(years[next]!, true)
    }
  }
  const onPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) {
      return
    }
    event.preventDefault()
    // Preventing the press's default also skips its focus, so the arrow keys would still go elsewhere.
    event.currentTarget.focus({ preventScroll: true })
    event.currentTarget.setPointerCapture?.(event.pointerId)
    drag.current = { id: event.pointerId, y0: event.clientY, from: at }
  }
  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const d = drag.current
    if (!d || d.id !== event.pointerId) {
      return
    }
    const next = Math.min(years.length - 1, Math.max(0, d.from + Math.round((event.clientY - d.y0) / dragPxPerYear)))
    if (years[next] !== current) {
      onPick(years[next]!, false)
    }
  }
  const release = () => {
    drag.current = null
  }
  const onKeyDown = (event: KeyboardEvent) => {
    const to = { ArrowUp: at - 1, ArrowDown: at + 1, Home: 0, End: years.length - 1 }[event.key]
    if (to !== undefined) {
      event.preventDefault()
      step(to - at)
    }
  }

  return (
    <div className={styles.handleColumn}>
      <div role="group" aria-label="Move through the years" className={styles.handle}>
        <button type="button" aria-label="Newer year" disabled={at === 0} onClick={() => step(-1)} className={styles.handleStep}>
          ▲
        </button>
        <div
          role="slider"
          tabIndex={0}
          aria-label="Year"
          aria-orientation="vertical"
          aria-valuemin={0}
          aria-valuemax={years.length - 1}
          aria-valuenow={at}
          aria-valuetext={labelOf(current, true)}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={release}
          onPointerCancel={release}
          onKeyDown={onKeyDown}
          className={styles.grip}
        >
          <AnimatePresence mode="popLayout" initial={false}>
            <m.span key={current} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -8 }} transition={{ duration: 0.14 }}>
              {labelOf(current, false)}
            </m.span>
          </AnimatePresence>
        </div>
        <button
          type="button"
          aria-label="Older year"
          disabled={at === years.length - 1}
          onClick={() => step(1)}
          className={styles.handleStep}
        >
          ▼
        </button>
      </div>
    </div>
  )
}

export const undated = 'undated'

function labelOf(year: string, spoken: boolean) {
  if (year === undated) {
    return spoken ? 'Undated' : '—'
  }
  return spoken ? year : `’${year.slice(2)}`
}
