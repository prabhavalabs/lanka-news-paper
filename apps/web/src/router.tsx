import type { ComponentType } from 'react'
import { createBrowserRouter } from 'react-router'

import { RootLayout } from './pages/root-layout'
import { NotFoundPage, RouteErrorPage } from './pages/route-error-page'
import { StaticPage } from './pages/static-page'

function RouteLoading() {
  return (
    <main className="grid min-h-screen place-items-center bg-background text-sm text-muted-foreground">
      <p role="status">පිටුව පූරණය වෙමින්…</p>
    </main>
  )
}

const lazyPage = <T extends Record<string, unknown>, K extends keyof T>(loader: () => Promise<T>, exportName: K) => async () => {
  const module = await loader()
  return { Component: module[exportName] as ComponentType }
}

const lazyFeedPage = (mode: 'category' | 'source' | 'search') => async () => {
  const { FeedPage } = await import('./pages/feed-page')
  return { Component: () => <FeedPage mode={mode} /> }
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    HydrateFallback: RouteLoading,
    errorElement: <RouteErrorPage />,
    children: [
      { index: true, lazy: lazyPage(() => import('./pages/front-page'), 'FrontPage') },
      { path: 'c/:slug', lazy: lazyFeedPage('category') },
      { path: 'sources', lazy: lazyPage(() => import('./pages/sources-page'), 'SourcesPage') },
      { path: 'sources/:id', lazy: lazyFeedPage('source') },
      { path: 'search', lazy: lazyFeedPage('search') },
      { path: 'a/:id', lazy: lazyPage(() => import('./pages/article-page'), 'ArticlePage') },
      { path: 'e/:id', lazy: lazyPage(() => import('./pages/event-page'), 'EventPage') },
      { path: 'analysis/knowledge', lazy: lazyPage(() => import('./pages/knowledge-analysis-page'), 'KnowledgeAnalysisPage') },
      { path: 'brief', lazy: lazyPage(() => import('./pages/brief-page'), 'BriefPage') },
      {
        path: 'about',
        element: (
          <StaticPage
            title="අප ගැන"
            body="මෙය සිංහල පුවත් සොයාගැනීමේ වේදිකාවකි. සම්පූර්ණ ලිපි මෙහි නැවත පළ නොවේ; මුල් ප්‍රකාශකයා වෙත යොමු වන්න."
          />
        ),
      },
      {
        path: 'privacy',
        element: (
          <StaticPage
            title="රහස්‍යතාව"
            body="පොදු වෙබ් අඩවිය කියවීමට ගිණුමක් අවශ්‍ය නැත. එකතු කරන දත්ත අවමයි."
          />
        ),
      },
      {
        path: 'corrections',
        element: (
          <StaticPage
            title="නිවැරදි කිරීම්"
            body="ප්‍රකාශක නිවැරදි කිරීම් සහ ඉවත් කිරීම් මිනිත්තු පහක් ඇතුළත පොදු දසුනට යොමු විය යුතුය."
          />
        ),
      },
      {
        path: 'contact',
        element: (
          <StaticPage
            title="සම්බන්ධ වන්න"
            body="පැමිණිලි සඳහා ලිපියේ “ගැටලුවක් වාර්තා කරන්න” භාවිතා කරන්න."
          />
        ),
      },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
])
