import type { ReactNode } from 'react'
import { Navigate, type RouteObject } from 'react-router'

import { SignInLayout } from '../components/SignInLayout/SignInLayout'
import { FirstSetup } from '../pages/FirstSetup/FirstSetup'
import { ForgotPassword } from '../pages/ForgotPassword/ForgotPassword'
import { Account } from '../pages/Account/Account'
import { EventPage } from '../pages/Events/EventPage'
import { Events } from '../pages/Events/Events'
import { HomeHearth } from '../pages/HomeHearth/HomeHearth'
import { Login } from '../pages/Login/Login'
import { Main } from '../pages/Main/Main'
import { ManageAccess } from '../pages/ManageAccess/ManageAccess'
import { Members } from '../pages/Members/Members'
import { MyProfile } from '../pages/MyProfile/MyProfile'
import { PersonProfile } from '../pages/PersonProfile/PersonProfile'
import { RequestAccess } from '../pages/RequestAccess/RequestAccess'
import { Requests } from '../pages/Requests/Requests'
import { WelcomeInvite } from '../pages/WelcomeInvite/WelcomeInvite'
import { AppShell } from './AppShell'
import { NotBuiltYet } from './NotBuiltYet'
import { adminPages, menuPages } from './signedInPages'

const built: Record<string, ReactNode> = {
  '/': <HomeHearth />,
  '/tree': <Main />,
  '/members': <Members />,
  '/events': <Events />,
  '/admin/members': <ManageAccess />,
}

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
      { path: '/people/:id', element: <PersonProfile /> },
      { path: '/events/:id', element: <EventPage /> },
      { path: '/profile', element: <MyProfile /> },
      { path: '/requests', element: <Requests /> },
      { path: '/account', element: <Account /> },
      // Members replaced both (design-notes §4.3); old bookmarks land there.
      { path: '/people', element: <Navigate to="/members" replace /> },
      { path: '/directory', element: <Navigate to="/members" replace /> },
    ],
  },
  { path: '*', element: <NotBuiltYet /> },
]
