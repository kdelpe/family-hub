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

describe('AskButton', () => {
  it('doesn’t mark an ask as made for whoever signed in while it was on its way', async () => {
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

    // Another person signs in and loads the directory before the answer arrives.
    act(() => {
      endSession(client)
      client.setQueryData(directoryKeys.all, [entry])
    })
    await act(async () => answer())

    expect(client.getQueryData<DirectoryEntry[]>(directoryKeys.all)![0]!.contacts).toEqual(entry.contacts)
    expect(screen.getByRole('button', { name: 'Request address' })).toBeInTheDocument()
    expect(screen.queryByText('Address requested')).toBeNull()
  })
})
