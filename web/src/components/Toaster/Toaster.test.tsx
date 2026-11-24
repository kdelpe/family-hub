import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { type Note, Toaster, useToast } from './Toaster'

function Shows({ note }: { note: Note }) {
  const toast = useToast()
  return (
    <button type="button" onClick={() => toast(note)}>
      Show
    </button>
  )
}

function showNote(note: Note) {
  render(
    <Toaster>
      <Shows note={note} />
    </Toaster>,
  )
  fireEvent.click(screen.getByRole('button', { name: 'Show' }))
}

const wait = (ms: number) => act(() => vi.advanceTimersByTime(ms))

describe('Toaster', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('runs a note’s action once, when its five seconds are up and not before', () => {
    const onDone = vi.fn()
    showNote({ text: 'Removed “Wedding”.', onDone })

    wait(4990)
    expect(onDone).not.toHaveBeenCalled()
    wait(20)
    expect(onDone).toHaveBeenCalledTimes(1)
    wait(10000)
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('holds the deadline while the pointer is over it, then gives the time that was left', () => {
    const onDone = vi.fn()
    showNote({ text: 'Removed “Wedding”.', onDone })
    const notes = screen.getByRole('region', { name: /Notifications/ })

    wait(2000)
    fireEvent.pointerMove(notes)
    wait(10000)
    expect(onDone).not.toHaveBeenCalled()

    fireEvent.pointerLeave(notes)
    wait(2990)
    expect(onDone).not.toHaveBeenCalled()
    wait(20)
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('cancels the deadline on Undo', () => {
    const onDone = vi.fn()
    const undo = vi.fn()
    showNote({ text: 'Removed “Wedding”.', onDone, undo })

    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    wait(10000)

    expect(undo).toHaveBeenCalledTimes(1)
    expect(onDone).not.toHaveBeenCalled()
  })
})
