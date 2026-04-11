import React, { type ReactElement, useEffect, useState } from 'react';
import { useDispatch } from '@/store';
import { Box, CircularProgress, Stack, Typography, Button } from '@mui/material';
import { Refresh as RefreshIcon } from '@mui/icons-material';

import { services } from '@/services';
import { setUser } from '@/store/slices/user';
import ErrorContainer from '@/components/ErrorContainer';

interface FallbackComponentProps {
  errorMessage: string;
}

function FallbackComponent({ errorMessage }: FallbackComponentProps): ReactElement {
  return (
    <ErrorContainer>
      <Box sx={{ maxWidth: 480, width: '100%' }}>
        <Typography
          variant="h5"
          sx={{
            fontWeight: 600,
            color: 'text.primary',
          }}
        >
          Something Went Wrong
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
          {errorMessage ||
            'We encountered an issue. Please try again later or contact support if the problem persists.'}
        </Typography>

        <Stack direction="row" spacing={2} sx={{ mt: 4 }}>
          <Button
            variant="contained"
            color="secondary"
            onClick={() => window.location.reload()}
            startIcon={<RefreshIcon />}
          >
            Try Again
          </Button>
        </Stack>
      </Box>
    </ErrorContainer>
  );
}

interface AuthGuardProps {
  children: React.ReactElement;
}

const AuthGuard = ({ children }: AuthGuardProps): ReactElement => {
  const dispatch = useDispatch();
  const [isLoading, setLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState<string>('');

  useEffect(() => {
    const fetchData = async (): Promise<void> => {
      try {
        const user = await services.user.getMe();
        dispatch(setUser(user));
      } catch (err) {
        console.error('AuthGuard: failed to load user', err);
        setErrorMessage('Failed to retrieve user information. Please try again later.');
      } finally {
        setLoading(false);
      }
    };

    void fetchData();
  }, [dispatch]);

  if (isLoading) {
    return (
      <Stack direction="row" justifyContent="center" alignItems="center" sx={{ height: '100vh' }}>
        <CircularProgress />
      </Stack>
    );
  }

  if (errorMessage) {
    return <FallbackComponent errorMessage={errorMessage} />;
  }

  return <>{children}</>;
};

export default AuthGuard;
