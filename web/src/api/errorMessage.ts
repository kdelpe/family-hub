import { ApiError, networkErrorStatus } from './client'

const somethingWrong = 'Something went wrong. Try again in a moment.'

// The plain-language message for a failure any page can hit; pages handle their own codes (like 401) first.
export function errorMessage(error: Error): string {
  if (!(error instanceof ApiError)) {
    return somethingWrong
  }
  switch (error.status) {
    case 429: {
      const minutes = Math.max(1, Math.ceil((error.retryAfterSeconds ?? 60) / 60))
      return `Too many tries. Wait ${minutes} ${minutes === 1 ? 'minute' : 'minutes'} and try again.`
    }
    case 503:
      return 'The family hub is busy right now. Try again in a moment.'
    case networkErrorStatus:
      return 'Can’t reach the family hub. Check your connection and try again.'
    default:
      return somethingWrong
  }
}
