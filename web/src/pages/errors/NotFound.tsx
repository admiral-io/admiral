import type { JSX } from 'react';
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Stack, Typography, Button } from '@mui/material';
import { Home as HomeIcon, ArrowBack as ArrowBackIcon } from '@mui/icons-material';

export default function NotFoundPage(): JSX.Element {
  const navigate = useNavigate();

  useEffect(() => {
    // TODO: replace with Sentry.captureException when @sentry/react is added
    console.warn('Page not found:', {
      path: window.location.pathname,
      referrer: document.referrer || 'none',
    });
  }, []);

  const handleGoHome = () => {
    navigate('/');
  };

  const handleGoBack = () => {
    if (window.history.length > 1) {
      navigate(-1);
    } else {
      navigate('/');
    }
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
        404
      </Typography>

      <Typography
        variant="h5"
        sx={{
          mt: 2,
          fontWeight: 600,
          color: 'text.primary',
        }}
      >
        Page not found
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
        The page you're looking for doesn't exist or has been moved. Check the URL or head back to
        familiar territory.
      </Typography>

      <Stack direction="row" spacing={2} sx={{ mt: 4 }}>
        <Button
          variant="contained"
          color="secondary"
          onClick={handleGoHome}
          startIcon={<HomeIcon />}
        >
          Go Home
        </Button>

        <Button variant="outlined" onClick={handleGoBack} startIcon={<ArrowBackIcon />}>
          Go Back
        </Button>
      </Stack>
    </Box>
  );
}
