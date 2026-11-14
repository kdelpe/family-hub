import type { RouteObject } from 'react-router'

import { NotBuiltYet } from './NotBuiltYet'

// The only place routes are wired.
export const routes: RouteObject[] = [{ path: '*', element: <NotBuiltYet /> }]
