import { lazy } from 'react';
import { createBrowserRouter } from 'react-router-dom';

import AuthGuard from '@/routes/AuthGuard';
import AppLayout from '@/layouts/app';
import ErrorLayout from '@/layouts/Error';
import Loadable from '@/components/Loadable';

const FooPage = Loadable(lazy(() => import('@/pages/foo')));

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
        element: <FooPage />,
        handle: { title: 'Foo' },
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
