import { createBrowserRouter } from 'react-router'

import { ArticlePage } from './pages/article-page'
import { BriefPage } from './pages/brief-page'
import { EventPage } from './pages/event-page'
import { FeedPage } from './pages/feed-page'
import { FrontPage } from './pages/front-page'
import { KnowledgeAnalysisPage } from './pages/knowledge-analysis-page'
import { RootLayout } from './pages/root-layout'
import { NotFoundPage, RouteErrorPage } from './pages/route-error-page'
import { SourcesPage } from './pages/sources-page'
import { StaticPage } from './pages/static-page'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    errorElement: <RouteErrorPage />,
    children: [
      { index: true, element: <FrontPage /> },
      { path: 'c/:slug', element: <FeedPage mode="category" /> },
      { path: 'sources', element: <SourcesPage /> },
      { path: 'sources/:id', element: <FeedPage mode="source" /> },
      { path: 'search', element: <FeedPage mode="search" /> },
      { path: 'a/:id', element: <ArticlePage /> },
      { path: 'e/:id', element: <EventPage /> },
      { path: 'analysis/knowledge', element: <KnowledgeAnalysisPage /> },
      { path: 'brief', element: <BriefPage /> },
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
