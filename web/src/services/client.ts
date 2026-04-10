import createClient from 'openapi-fetch';

import type { paths } from '@/services/api';
import { parseApiError } from '@/services/errors';

// Empty baseUrl means requests are relative to the current origin.
export const client = createClient<paths>({
  baseUrl: import.meta.env.VITE_API_BASE_URL || '',
  credentials: 'include',
});

// Global response middleware: handle 401 redirects and parse gRPC error bodies.
client.use({
  async onResponse({ response }) {
    if (response.status === 401) {
      // Bail out if already on an auth page to prevent a redirect loop.
      if (window.location.pathname.startsWith('/auth/')) {
        throw new Error('Authentication failed');
      }

      const raw = window.location.pathname + window.location.search;
      const isSafe = raw.startsWith('/') && !raw.startsWith('//');
      const redirectUrl = isSafe ? raw : '/';

      window.location.href = `/auth/login?redirect_url=${encodeURIComponent(redirectUrl)}`;

      // Navigation is async; throw to prevent downstream code from running on a stale response.
      throw new Error('Redirecting to login');
    }

    if (!response.ok) {
      const body = await response
        .clone()
        .json()
        .catch(() => null);
      const apiError = parseApiError(body);
      if (apiError) {
        throw apiError;
      }
    }

    return response;
  },
});
