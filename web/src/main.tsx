import { QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'

import { followSessionChanges } from './api/auth'
import { createQueryClient } from './api/queryClient'
import { MotionPreferences } from './app/MotionPreferences'
import { routes } from './app/routes'
import './styles/fonts.css'
import './styles/tokens.css'
import './styles/base.css'

const queryClient = createQueryClient()
followSessionChanges(queryClient)
const router = createBrowserRouter(routes)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <MotionPreferences>
        <RouterProvider router={router} />
      </MotionPreferences>
    </QueryClientProvider>
  </StrictMode>,
)
