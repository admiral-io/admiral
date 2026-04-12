import { lazy } from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';

import AuthGuard from '@/routes/AuthGuard';
import AppLayout from '@/layouts/app';
import ErrorLayout from '@/layouts/Error';
import Loadable from '@/components/Loadable';

const LandingPage = Loadable(lazy(() => import('@/pages/landing')));

// In-progress pages — kept reachable by URL for development, but not linked
// from the sidebar while the UI is under construction.
const ApplicationsPage = Loadable(lazy(() => import('@/pages/applications/pages/ApplicationsPage')));
const ApplicationDetailPage = Loadable(lazy(() => import('@/pages/applications/pages/ApplicationDetailPage')));

const UserLayout = Loadable(lazy(() => import('@/pages/user')));
const ProfilePage = Loadable(lazy(() => import('@/pages/user/profile/ProfilePage')));
const TokensPage = Loadable(lazy(() => import('@/pages/user/tokens/pages/TokensPage')));
const TokenCreatePage = Loadable(lazy(() => import('@/pages/user/tokens/pages/TokenCreatePage')));

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
      {
        path: '/user',
        element: <UserLayout />,
        handle: { title: 'Account Settings' },
        children: [
          { index: true, element: <Navigate to="profile" replace /> },
          {
            path: 'profile',
            element: <ProfilePage />,
            handle: { title: 'Profile' },
          },
          {
            path: 'tokens',
            element: <TokensPage />,
            handle: { title: 'Personal Access Tokens' },
          },
          {
            path: 'tokens/new',
            element: <TokenCreatePage />,
            handle: { title: 'New Token' },
          },
        ],
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
