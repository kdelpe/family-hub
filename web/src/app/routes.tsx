import type { RouteObject } from 'react-router'

import { SignInLayout } from '../components/SignInLayout/SignInLayout'
import { FirstSetup } from '../pages/FirstSetup/FirstSetup'
import { ForgotPassword } from '../pages/ForgotPassword/ForgotPassword'
import { HomeHearth } from '../pages/HomeHearth/HomeHearth'
import { Login } from '../pages/Login/Login'
import { RequestAccess } from '../pages/RequestAccess/RequestAccess'
import { WelcomeInvite } from '../pages/WelcomeInvite/WelcomeInvite'
import { AppShell } from './AppShell'
import { NotBuiltYet } from './NotBuiltYet'

// Signed-in pages still to come; the menu already links to them.
const comingSoon = ['/tree', '/people', '/directory', '/events', '/stories', '/profile', '/admin/members']

// The only place routes are wired.
export const routes: RouteObject[] = [
  {
    element: <SignInLayout />,
    children: [
      { path: '/auth/login', element: <Login /> },
      { path: '/auth/request-access', element: <RequestAccess /> },
      { path: '/auth/forgot', element: <ForgotPassword /> },
      { path: '/auth/set-password', element: <WelcomeInvite /> },
      { path: '/setup', element: <FirstSetup /> },
    ],
  },
  {
    element: <AppShell />,
    children: [
      { path: '/', element: <HomeHearth /> },
      ...comingSoon.map((path) => ({ path, element: <NotBuiltYet /> })),
    ],
  },
  { path: '*', element: <NotBuiltYet /> },
]
