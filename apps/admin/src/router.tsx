import type { ComponentType } from 'react'
import { createBrowserRouter } from 'react-router'

import { NotFoundPage, RouteErrorPage } from './pages/route-error-page'
import { ShellLayout } from './pages/shell-layout'

function RouteLoading() {
  return (
    <main className="grid min-h-screen place-items-center bg-background text-sm text-muted-foreground">
      <p role="status">Loading page…</p>
    </main>
  )
}

const lazyPage = <T extends Record<string, unknown>, K extends keyof T>(loader: () => Promise<T>, exportName: K) => async () => {
  const module = await loader()
  return { Component: module[exportName] as ComponentType }
}

export const router = createBrowserRouter([
  {
    path: '/login',
    lazy: lazyPage(() => import('./pages/login-page'), 'LoginPage'),
    HydrateFallback: RouteLoading,
    errorElement: <RouteErrorPage />,
  },
  {
    path: '/',
    element: <ShellLayout />,
    HydrateFallback: RouteLoading,
    errorElement: <RouteErrorPage />,
    children: [
      { index: true, lazy: lazyPage(() => import('./pages/dashboard-page'), 'DashboardPage') },
      { path: 'sources', lazy: lazyPage(() => import('./pages/sources-page'), 'SourcesPage') },
      { path: 'sources/:id', lazy: lazyPage(() => import('./pages/source-detail-page'), 'SourceDetailPage') },
      { path: 'sources/:id/feed', lazy: lazyPage(() => import('./pages/source-feed-page'), 'SourceFeedPage') },
      { path: 'articles', lazy: lazyPage(() => import('./pages/articles-page'), 'ArticlesPage') },
      { path: 'articles/:id', lazy: lazyPage(() => import('./pages/article-detail-page'), 'ArticleDetailPage') },
      { path: 'knowledge', lazy: lazyPage(() => import('./pages/knowledge-graph-page'), 'KnowledgeGraphPage') },
      { path: 'jobs', lazy: lazyPage(() => import('./pages/jobs-page'), 'JobsPage') },
      { path: 'queue', lazy: lazyPage(() => import('./pages/queue-page'), 'QueuePage') },
      { path: 'complaints', lazy: lazyPage(() => import('./pages/complaints-page'), 'ComplaintsPage') },
      { path: 'routing', lazy: lazyPage(() => import('./pages/routing-page'), 'RoutingPage') },
      { path: 'workflows', lazy: lazyPage(() => import('./pages/workflows-page'), 'WorkflowsPage') },
      { path: 'settings', lazy: lazyPage(() => import('./pages/settings-page'), 'SettingsPage') },
      { path: 'watch-tower', lazy: lazyPage(() => import('./pages/watch-tower-page'), 'WatchTowerPage') },
      { path: 'mailing-list', lazy: lazyPage(() => import('./pages/mailing-list-page'), 'MailingListPage') },
    ],
  },
  { path: '*', element: <NotFoundPage />, errorElement: <RouteErrorPage /> },
])
