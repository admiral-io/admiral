import React from 'react';
import { Box, Stack, Typography } from '@mui/material';

import { Logo } from '@/components/Logo';

const LandingPage: React.FC = () => {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '70vh',
      }}
    >
      <Stack spacing={3} alignItems="center" sx={{ maxWidth: 520, textAlign: 'center' }}>
        <Logo width={288} height={80} />

        <Typography
          variant="h6"
          sx={{
            fontWeight: 500,
            color: 'text.primary',
          }}
        >
          The web UI is under active development.
        </Typography>

        <Typography
          variant="body1"
          sx={{
            color: 'text.secondary',
            lineHeight: 1.6,
          }}
        >
          For now, use the CLI or API to manage your applications, environments, and deployments.
          This console will fill out as features land.
        </Typography>
      </Stack>
    </Box>
  );
};

export default LandingPage;