import { lazy } from 'react';
import { createBrowserRouter } from 'react-router-dom';

import AuthGuard from '@/routes/AuthGuard';
import AppLayout from '@/layouts/app';
import ErrorLayout from '@/layouts/Error';
import Loadable from '@/components/Loadable';

const LandingPage = Loadable(lazy(() => import('@/pages/landing')));

// In-progress pages — kept reachable by URL for development, but not linked
// from the sidebar while the UI is under construction.
const ApplicationsPage = Loadable(lazy(() => import('@/pages/applications')));
const ApplicationDetailPage = Loadable(lazy(() => import('@/pages/applications/ApplicationDetailPage')));

const NotFound = Loadable(lazy(() => import('@/pages/errors/NotFound')));
const AuthError = Loadable(lazy(() => import('@/pages/errors/AuthError')));

const router = createBrowserRouter([
  {
    element: (
      <AuthGuard>
        <AppLayout />
      </AuthGuard>
    ),
    children: [
      {
        path: '/',
        element: <LandingPage />,
        handle: { title: 'Admiral' },
      },
      {
        path: '/applications',
        element: <ApplicationsPage />,
        handle: { title: 'Applications' },
      },
      {
        path: '/applications/:applicationId',
        element: <ApplicationDetailPage />,
        handle: { title: 'Application' },
      },
    ],
  },
  {
    element: <ErrorLayout />,
    children: [
      {
        path: '/error',
        element: <AuthError />,
        handle: { title: 'Authentication Error' },
      },
      {
        path: '*',
        element: <NotFound />,
        handle: { title: 'Not Found' },
      },
    ],
  },
]);

export default router;
