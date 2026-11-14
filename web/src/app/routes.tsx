import type { RouteObject } from 'react-router'

import { NotBuiltYet } from './NotBuiltYet'

// The only place routes are wired; the pages before sign-in come next.
export const routes: RouteObject[] = [{ path: '*', element: <NotBuiltYet /> }]
