// The one error every API call throws: the server's {error: {code, message}}, or a stand-in when there is none.
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly retryAfterSeconds: number | null

  constructor(status: number, code: string, message: string, retryAfterSeconds: number | null = null) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.retryAfterSeconds = retryAfterSeconds
  }
}

// status 0: the request never got an answer (offline, server down).
export const networkErrorStatus = 0

type Method = 'GET' | 'POST' | 'PATCH' | 'DELETE'

type Options = {
  body?: unknown
  // TanStack Query's, so a query it cancels stops its request too.
  signal?: AbortSignal
}

export async function api<T>(method: Method, path: string, { body, signal }: Options = {}): Promise<T> {
  // Outside the try: a body that can't be serialized is a bug, not a lost connection.
  const json = body === undefined ? undefined : JSON.stringify(body)
  let response: Response
  try {
    response = await fetch(`/api${path}`, {
      method,
      headers: json === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: json,
      signal,
    })
  } catch (error) {
    if (signal?.aborted) {
      throw error
    }
    throw new ApiError(networkErrorStatus, 'network_error', 'no answer from the server')
  }
  if (!response.ok) {
    throw await errorFrom(response)
  }
  if (response.status === 204 || response.status === 202) {
    return undefined as T
  }
  try {
    return (await response.json()) as T
  } catch {
    throw new ApiError(response.status, 'bad_response', 'the server’s answer wasn’t JSON')
  }
}

async function errorFrom(response: Response): Promise<ApiError> {
  const retryAfter = Number(response.headers.get('Retry-After'))
  const retryAfterSeconds = Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : null
  try {
    const { error } = (await response.json()) as { error: { code: string; message: string } }
    return new ApiError(response.status, error.code, error.message, retryAfterSeconds)
  } catch {
    return new ApiError(response.status, 'unknown', response.statusText, retryAfterSeconds)
  }
}
