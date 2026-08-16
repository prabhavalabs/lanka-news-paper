import { createBrowserRouter } from 'react-router'

import { FrontPage } from './pages/front-page'
import { RootLayout } from './pages/root-layout'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    children: [{ index: true, element: <FrontPage /> }],
  },
])
