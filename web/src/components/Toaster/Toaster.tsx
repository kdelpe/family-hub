import * as Toast from '@radix-ui/react-toast'
import { AnimatePresence, m } from 'motion/react'
import { createContext, type ReactNode, useCallback, useContext, useEffect, useRef, useState } from 'react'

import { onSessionChange } from '../../api/auth'
import { useMotionPaused } from '../../lib/motion'
import styles from './Toaster.module.css'

export type Note = {
  text: string
  // Shows an Undo button; the note is done without running onDone.
  undo?: () => void
  // When it times out, is swiped away or makes room for the next note.
  onDone?: () => void
}

const showDuration = 5000

const ShowContext = createContext<(note: Note) => void>(() => {})

export const useToast = () => useContext(ShowContext)

export function Toaster({ children }: { children: ReactNode }) {
  const [shown, setShown] = useState<(Note & { id: number }) | null>(null)
  const [paused, setPaused] = useState(false)
  // A ref, so a late close or a second close for the same note does nothing.
  const current = useRef<(Note & { id: number }) | null>(null)
  const count = useRef(0)
  const [motionPaused] = useMotionPaused()

  // By id: a note fading out after a newer one replaced it keeps its handlers, which mustn't act on the new one.
  const finish = useCallback((id: number | undefined, undo: boolean) => {
    const note = current.current
    if (!note || (id !== undefined && note.id !== id)) {
      return
    }
    current.current = null
    setShown(null)
    if (undo) {
      note?.undo?.()
    } else {
      note?.onDone?.()
    }
  }, [])

  // A note is the signed-in person's: it goes the moment they sign out or someone else signs in, its action
  // (a removal's DELETE) checking sameSession and so sending nothing.
  useEffect(() => onSessionChange(() => finish(undefined, false)), [finish])

  const show = useCallback(
    (note: Note) => {
      finish(undefined, false)
      count.current += 1
      current.current = { ...note, id: count.current }
      setPaused(false)
      setShown(current.current)
    },
    [finish],
  )

  return (
    <ShowContext.Provider value={show}>
      <Toast.Provider duration={showDuration} swipeDirection="down">
        {children}
        <AnimatePresence>
          {shown && (
            <Toast.Root
              key={shown.id}
              forceMount
              open
              onOpenChange={(open) => !open && finish(shown.id, false)}
              onPause={() => setPaused(true)}
              onResume={() => setPaused(false)}
              asChild
            >
              <m.li
                className={styles.toast}
                initial={{ y: 80, opacity: 0 }}
                animate={{ y: 0, opacity: 1 }}
                exit={{ y: 40, opacity: 0 }}
                transition={{ type: 'spring', stiffness: 420, damping: 28 }}
              >
                <Toast.Description>{shown.text}</Toast.Description>
                {shown.undo && (
                  <Toast.Action altText="Undo" asChild>
                    <button type="button" onClick={() => finish(shown.id, true)} className={styles.undo}>
                      Undo
                    </button>
                  </Toast.Action>
                )}
                {!motionPaused && (
                  <span
                    aria-hidden="true"
                    className={styles.bar}
                    style={{ animationDuration: `${showDuration}ms`, animationPlayState: paused ? 'paused' : 'running' }}
                  />
                )}
              </m.li>
            </Toast.Root>
          )}
        </AnimatePresence>
        <Toast.Viewport className={styles.viewport} />
      </Toast.Provider>
    </ShowContext.Provider>
  )
}
