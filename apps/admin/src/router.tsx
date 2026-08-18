import { createBrowserRouter } from 'react-router'

import { ComplaintsPage } from './pages/complaints-page'
import { ArticleDetailPage } from './pages/article-detail-page'
import { ArticlesPage } from './pages/articles-page'
import { DashboardPage } from './pages/dashboard-page'
import { LoginPage } from './pages/login-page'
import { KnowledgeGraphPage } from './pages/knowledge-graph-page'
import { QueuePage } from './pages/queue-page'
import { RoutingPage } from './pages/routing-page'
import { ShellLayout } from './pages/shell-layout'
import { SourceDetailPage } from './pages/source-detail-page'
import { SourcesPage } from './pages/sources-page'

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    path: '/',
    element: <ShellLayout />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: 'sources', element: <SourcesPage /> },
      { path: 'sources/:id', element: <SourceDetailPage /> },
      { path: 'articles', element: <ArticlesPage /> },
      { path: 'articles/:id', element: <ArticleDetailPage /> },
      { path: 'knowledge', element: <KnowledgeGraphPage /> },
      { path: 'queue', element: <QueuePage /> },
      { path: 'complaints', element: <ComplaintsPage /> },
      { path: 'routing', element: <RoutingPage /> },
    ],
  },
])
