import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { endSession } from '../../api/auth'
import { type DirectoryEntry, directoryKeys } from '../../api/contacts'
import { AskButton } from './AskButton'

const entry: DirectoryEntry = {
  id: 'nadege',
  first_name: 'Nadège',
  last_name: 'Pluviose',
  side: 'both',
  has_account: true,
  contacts: [{ kind: 'address', requestable: true, my_request_status: 'none' }],
}

afterEach(() => {
  vi.unstubAllGlobals()
})

// Asks for Nadège's address with the answer held back; returns the client and a function that lets the answer through.
async function askAndHold() {
  let answer!: () => void
  vi.stubGlobal(
    'fetch',
    vi.fn(
      () =>
        new Promise<Response>((resolve) => {
          answer = () => resolve(new Response(JSON.stringify({ kind: 'address', my_request_status: 'requested' }), { status: 200 }))
        }),
    ),
  )
  const client = new QueryClient()
  client.setQueryData(directoryKeys.all, [entry])
  render(
    <QueryClientProvider client={client}>
      <AskButton personId="nadege" kind="address" asked={false} size="small" />
    </QueryClientProvider>,
  )
  fireEvent.click(screen.getByRole('button', { name: 'Request address' }))
  await waitFor(() => expect(fetch).toHaveBeenCalled())
  const settle = async () => {
    await act(async () => answer())
    await waitFor(() => expect(client.isMutating()).toBe(0))
  }
  return { client, answer: settle }
}

describe('AskButton', () => {
  it('doesn’t mark an ask as made for whoever signed in while it was on its way', async () => {
    const { client, answer } = await askAndHold()

    // Another person signs in and loads the directory before the answer arrives.
    act(() => {
      endSession(client)
      client.setQueryData(directoryKeys.all, [entry])
    })
    await answer()

    expect(client.getQueryData<DirectoryEntry[]>(directoryKeys.all)![0]!.contacts).toEqual(entry.contacts)
    expect(screen.getByRole('button', { name: 'Request address' })).toBeInTheDocument()
    expect(screen.queryByText('Address requested')).toBeNull()
  })

  it('leaves a directory whose refresh failed hidden, rather than marking it fetched', async () => {
    const { client, answer } = await askAndHold()
    await client.fetchQuery({ queryKey: directoryKeys.all, queryFn: () => Promise.reject(new Error('offline')) }).catch(() => {})
    expect(client.getQueryState(directoryKeys.all)?.status).toBe('error')

    await answer()

    expect(client.getQueryState(directoryKeys.all)?.status).toBe('error')
    expect(screen.getAllByText('Address requested')).not.toEqual([])
  })
})
