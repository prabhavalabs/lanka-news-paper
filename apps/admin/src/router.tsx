import { createBrowserRouter } from 'react-router'

import { LoginPage } from './pages/login-page'
import { ShellLayout } from './pages/shell-layout'
import { SourcesPage } from './pages/sources-page'

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    path: '/',
    element: <ShellLayout />,
    children: [{ index: true, element: <SourcesPage /> }],
  },
])
