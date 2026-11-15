import type { RouteObject } from 'react-router'

import { SignInLayout } from '../components/SignInLayout/SignInLayout'
import { ForgotPassword } from '../pages/ForgotPassword/ForgotPassword'
import { Login } from '../pages/Login/Login'
import { NotBuiltYet } from './NotBuiltYet'

// The only place routes are wired.
export const routes: RouteObject[] = [
  {
    element: <SignInLayout />,
    children: [
      { path: '/auth/login', element: <Login /> },
      { path: '/auth/forgot', element: <ForgotPassword /> },
    ],
  },
  { path: '*', element: <NotBuiltYet /> },
]
