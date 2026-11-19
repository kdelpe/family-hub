import type { ReactNode } from 'react'
import type { RouteObject } from 'react-router'

import { SignInLayout } from '../components/SignInLayout/SignInLayout'
import { FirstSetup } from '../pages/FirstSetup/FirstSetup'
import { ForgotPassword } from '../pages/ForgotPassword/ForgotPassword'
import { HomeHearth } from '../pages/HomeHearth/HomeHearth'
import { Login } from '../pages/Login/Login'
import { ManageAccess } from '../pages/ManageAccess/ManageAccess'
import { RequestAccess } from '../pages/RequestAccess/RequestAccess'
import { WelcomeInvite } from '../pages/WelcomeInvite/WelcomeInvite'
import { AppShell } from './AppShell'
import { NotBuiltYet } from './NotBuiltYet'
import { adminPages, menuPages } from './signedInPages'

const built: Record<string, ReactNode> = { '/': <HomeHearth />, '/admin/members': <ManageAccess /> }

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
      ...[...menuPages, ...adminPages].map(({ path }) => ({ path, element: built[path] ?? <NotBuiltYet /> })),
    ],
  },
  { path: '*', element: <NotBuiltYet /> },
]
