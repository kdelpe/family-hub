import * as Dialog from '@radix-ui/react-dialog'
import { AnimatePresence, m } from 'motion/react'
import { useRef } from 'react'

import styles from './ConfirmDialog.module.css'

type Props = {
  open: boolean
  question: string
  note?: string
  yes: string
  no: string
  busy: boolean
  error?: string
  onYes: () => void
  onCancel: () => void
  // Called after the fade-out, so it should read refs, not props.
  focusAfterClose?: () => HTMLElement | null
}

export function ConfirmDialog({ open, question, note, yes, no, busy, error, onYes, onCancel, focusAfterClose }: Props) {
  const yesRef = useRef<HTMLButtonElement>(null)
  // Radix only returns focus to its own Trigger, which this doesn't use.
  const opener = useRef<Element | null>(null)

  return (
    <Dialog.Root open={open} onOpenChange={(next) => !next && !busy && onCancel()}>
      <AnimatePresence>
        {open && (
          <Dialog.Portal forceMount>
            <Dialog.Overlay asChild forceMount>
              <m.div className={styles.scrim} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} />
            </Dialog.Overlay>
            <Dialog.Content
              asChild
              forceMount
              // Without a note, tells Radix there's no description.
              {...(note ? {} : { 'aria-describedby': undefined })}
              onOpenAutoFocus={(event) => {
                event.preventDefault()
                opener.current = document.activeElement
                yesRef.current?.focus()
              }}
              onCloseAutoFocus={(event) => {
                event.preventDefault()
                // Lost focus goes back to the opener; focus the page has moved on purpose stays put.
                const lost = !document.activeElement || document.activeElement === document.body
                const target = focusAfterClose?.() ?? (lost ? opener.current : null)
                if (target instanceof HTMLElement && target.isConnected) {
                  target.focus()
                }
              }}
            >
              <m.div
                className={styles.dialog}
                initial={{ opacity: 0, scale: 0.92, y: 14 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.96, y: 8 }}
                transition={{ type: 'spring', stiffness: 420, damping: 26 }}
              >
                <Dialog.Title className={styles.question}>{question}</Dialog.Title>
                {note ? <Dialog.Description className={styles.note}>{note}</Dialog.Description> : null}
                {error && (
                  <p role="alert" className={styles.error}>
                    {error}
                  </p>
                )}
                <div className={styles.choices}>
                  <button type="button" onClick={() => !busy && onCancel()} aria-disabled={busy || undefined} className={`pill ${styles.no}`}>
                    {no}
                  </button>
                  <button
                    ref={yesRef}
                    type="button"
                    onClick={() => !busy && onYes()}
                    aria-disabled={busy || undefined}
                    className={`pill ${styles.yes}`}
                  >
                    {yes}
                  </button>
                </div>
              </m.div>
            </Dialog.Content>
          </Dialog.Portal>
        )}
      </AnimatePresence>
    </Dialog.Root>
  )
}
