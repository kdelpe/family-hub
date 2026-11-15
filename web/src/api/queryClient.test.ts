import { expect, it } from 'vitest'

import { ApiError, networkErrorStatus } from './client'
import { shouldRetry } from './queryClient'

it.each([
  { name: 'a refusal is final', error: new ApiError(401, 'invalid_credentials', 'x'), failures: 0, want: false },
  { name: 'too many requests is final', error: new ApiError(429, 'rate_limited', 'x'), failures: 0, want: false },
  { name: 'a server failure is retried', error: new ApiError(503, 'busy', 'x'), failures: 0, want: true },
  { name: 'no answer is retried', error: new ApiError(networkErrorStatus, 'network_error', 'x'), failures: 1, want: true },
  { name: 'retries stop after two', error: new ApiError(503, 'busy', 'x'), failures: 2, want: false },
  { name: 'a bug in the page is final', error: new TypeError('x'), failures: 0, want: false },
])('$name', ({ error, failures, want }) => {
  expect(shouldRetry(failures, error)).toBe(want)
})
