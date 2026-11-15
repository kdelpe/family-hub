import { QueryClient } from '@tanstack/react-query'

import { ApiError, networkErrorStatus } from './client'

const maxRetries = 2

// Only a server or network failure might pass on a retry: not a 4xx answer, and not a bug in the page.
export function shouldRetry(failures: number, error: Error): boolean {
  return failures < maxRetries && error instanceof ApiError && (error.status >= 500 || error.status === networkErrorStatus)
}

export const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: shouldRetry },
      // Not paused while the browser says it's offline: the request fails, and the page says so.
      mutations: { networkMode: 'always' },
    },
  })
