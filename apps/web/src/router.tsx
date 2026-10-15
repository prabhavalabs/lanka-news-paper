import type { ComponentType } from 'react'
import { createBrowserRouter } from 'react-router'

import { RootLayout } from './pages/root-layout'
import { NotFoundPage, RouteErrorPage } from './pages/route-error-page'
import { AboutPage, StaticPage } from './pages/static-page'

function RouteLoading() {
  return (
    <main className="grid min-h-screen place-items-center bg-paper text-ink">
      <p role="status" className="font-headline text-[1.125rem] font-bold">
        තුලාව පූරණය වෙමින්…
      </p>
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
      { path: 's/:slug', lazy: lazyPage(() => import('./pages/storyline-page'), 'StorylinePage') },
      { path: 'e/:id', lazy: lazyPage(() => import('./pages/event-page'), 'EventPage') },
      { path: 'analysis/knowledge', lazy: lazyPage(() => import('./pages/knowledge-analysis-page'), 'KnowledgeAnalysisPage') },
      { path: 'brief', lazy: lazyPage(() => import('./pages/brief-page'), 'BriefPage') },
      { path: 'editorial', lazy: lazyPage(() => import('./pages/editorial-pages'), 'EditorialListPage') },
      { path: 'editorial/:id', lazy: lazyPage(() => import('./pages/editorial-pages'), 'EditorialPage') },
      { path: 'about', element: <AboutPage /> },
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
