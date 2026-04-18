import type { JSX } from 'react';
import { useEffect, useMemo, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Box, Stack, Typography, Button, Alert } from '@mui/material';
import { Home as HomeIcon, Refresh as RefreshIcon } from '@mui/icons-material';

import {
  ALLOWED_SSO_ERRORS,
  AUTO_RETRY_ERRORS,
  extractErrorType,
} from '@/pages/errors/authErrorMessages';

const RETRY_COOLDOWN_MS = 8_000;
const STORAGE_KEY = 'admiral_auth_retry_ts';

function canAutoRetry(): boolean {
  try {
    const lastAttempt = sessionStorage.getItem(STORAGE_KEY);
    if (!lastAttempt) return true;
    return Date.now() - Number(lastAttempt) > RETRY_COOLDOWN_MS;
  } catch {
    return false;
  }
}

function markRetryAttempt() {
  try {
    sessionStorage.setItem(STORAGE_KEY, String(Date.now()));
  } catch {
    // sessionStorage unavailable — skip
  }
}

function buildLoginUrl(searchParams: URLSearchParams): string {
  const raw = searchParams.get('redirect_url');
  const postLogin =
    raw && raw.startsWith('/') && !raw.startsWith('//') ? raw : '/';
  return `/auth/login?redirect_url=${encodeURIComponent(postLogin)}`;
}

export default function AuthErrorPage(): JSX.Element {
  const [searchParams] = useSearchParams();
  const searchKey = searchParams.toString();
  const autoRedirectIssuedForKeyRef = useRef<string | null>(null);

  const { errorType, rawMessage, isUnknown, message } = useMemo(() => {
    const params = new URLSearchParams(searchKey);
    const extracted = extractErrorType(params);
    const resolvedMessage = extracted.errorType
      ? ALLOWED_SSO_ERRORS[extracted.errorType]
      : 'An authentication error occurred. Please try logging in again.';
    return {
      errorType: extracted.errorType,
      rawMessage: extracted.rawMessage,
      isUnknown: extracted.isUnknown,
      message: resolvedMessage,
    };
  }, [searchKey]);

  useEffect(() => {
    console.warn('Authentication error:', {
      errorType: errorType || 'unknown',
      unknownQuery: isUnknown,
      rawMessage,
    });

    const loginUrl = buildLoginUrl(new URLSearchParams(searchKey));

    if (errorType && AUTO_RETRY_ERRORS.has(errorType) && canAutoRetry()) {
      if (autoRedirectIssuedForKeyRef.current === searchKey) {
        return;
      }
      autoRedirectIssuedForKeyRef.current = searchKey;
      markRetryAttempt();
      window.location.replace(loginUrl);
    }
  }, [searchKey, errorType, rawMessage, isUnknown]);

  const loginUrl = buildLoginUrl(new URLSearchParams(searchKey));

  const handleRetryLogin = () => {
    markRetryAttempt();
    window.location.href = loginUrl;
  };

  const handleGoHome = () => {
    window.location.href = '/';
  };

  return (
    <Box sx={{ maxWidth: 480, width: '100%' }}>
      <Typography
        sx={{
          fontSize: { xs: '4rem', sm: '6rem' },
          fontWeight: 800,
          lineHeight: 1,
          color: 'primary.main',
          letterSpacing: '-0.02em',
        }}
      >
        401
      </Typography>

      <Typography
        variant="h5"
        sx={{
          mt: 2,
          fontWeight: 600,
          color: 'text.primary',
        }}
      >
        Authentication Error
      </Typography>

      <Typography
        variant="body1"
        sx={{
          mt: 1.5,
          color: 'text.secondary',
          lineHeight: 1.6,
          maxWidth: 400,
        }}
      >
        {message}
      </Typography>

      {isUnknown && (
        <Alert severity="warning" sx={{ mt: 2 }}>
          This error text didn&apos;t match a known sign-in error, so details aren&apos;t shown here.
          Context is written to the browser console for developers—nothing is sent to Admiral servers
          from this page.
        </Alert>
      )}

      {(errorType === 'invalid_token' || errorType === 'invalid_grant') && (
        <Alert severity="info" sx={{ mt: 2 }}>
          Another login attempt does not always fix this: the token or grant may be unusable for other
          reasons (expiry, redirect mismatch, revoked refresh, or client configuration). Try clearing this
          site&apos;s data or contact your administrator if it keeps happening.
        </Alert>
      )}

      <Stack direction="row" spacing={2} sx={{ mt: 4 }}>
        <Button
          variant="contained"
          color="secondary"
          onClick={handleRetryLogin}
          startIcon={<RefreshIcon />}
        >
          {errorType === 'invalid_token' || errorType === 'invalid_grant'
            ? 'Open login page'
            : 'Try Login Again'}
        </Button>

        <Button variant="outlined" onClick={handleGoHome} startIcon={<HomeIcon />}>
          Go Home
        </Button>
      </Stack>
    </Box>
  );
}
