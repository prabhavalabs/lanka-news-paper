import { createBrowserRouter } from 'react-router'

import { ComplaintsPage } from './pages/complaints-page'
import { ArticleDetailPage } from './pages/article-detail-page'
import { ArticlesPage } from './pages/articles-page'
import { DashboardPage } from './pages/dashboard-page'
import { LoginPage } from './pages/login-page'
import { MailingListPage } from './pages/mailing-list-page'
import { KnowledgeGraphPage } from './pages/knowledge-graph-page'
import { JobsPage } from './pages/jobs-page'
import { QueuePage } from './pages/queue-page'
import { NotFoundPage, RouteErrorPage } from './pages/route-error-page'
import { RoutingPage } from './pages/routing-page'
import { SettingsPage } from './pages/settings-page'
import { ShellLayout } from './pages/shell-layout'
import { SourceDetailPage } from './pages/source-detail-page'
import { SourceFeedPage } from './pages/source-feed-page'
import { SourcesPage } from './pages/sources-page'
import { WatchTowerPage } from './pages/watch-tower-page'
import { WorkflowsPage } from './pages/workflows-page'

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage />, errorElement: <RouteErrorPage /> },
  {
    path: '/',
    element: <ShellLayout />,
    errorElement: <RouteErrorPage />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: 'sources', element: <SourcesPage /> },
      { path: 'sources/:id', element: <SourceDetailPage /> },
      { path: 'sources/:id/feed', element: <SourceFeedPage /> },
      { path: 'articles', element: <ArticlesPage /> },
      { path: 'articles/:id', element: <ArticleDetailPage /> },
      { path: 'knowledge', element: <KnowledgeGraphPage /> },
      { path: 'jobs', element: <JobsPage /> },
      { path: 'queue', element: <QueuePage /> },
      { path: 'complaints', element: <ComplaintsPage /> },
      { path: 'routing', element: <RoutingPage /> },
      { path: 'workflows', element: <WorkflowsPage /> },
      { path: 'settings', element: <SettingsPage /> },
      { path: 'watch-tower', element: <WatchTowerPage /> },
      { path: 'mailing-list', element: <MailingListPage /> },
    ],
  },
  { path: '*', element: <NotFoundPage />, errorElement: <RouteErrorPage /> },
])
