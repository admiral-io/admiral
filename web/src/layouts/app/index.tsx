import type { JSX } from 'react';
import { Box, useTheme } from '@mui/material';
import { Outlet } from 'react-router-dom';

import Sidebar from '@/layouts/app/Sidebar';
import Header from '@/layouts/app/Header';

const AppLayout = (): JSX.Element => {
  const theme = useTheme();

  return (
    <Box
      sx={{
        display: 'flex',
        minHeight: '100vh',
        background:
          theme.palette.mode === 'dark'
            ? `linear-gradient(135deg, ${theme.palette.background.default} 0%, ${theme.palette.background.paper} 100%)`
            : `linear-gradient(135deg, ${theme.palette.grey[50]} 0%, ${theme.palette.grey[100]} 100%)`,
      }}
    >
      <Sidebar />
      <Box
        component="main"
        role="main"
        aria-label="Main content"
        sx={{
          display: 'flex',
          flexDirection: 'column',
          flex: 1,
          overflow: 'hidden',
        }}
      >
        <Header />
        <Box
          sx={{
            flex: 1,
            overflow: 'auto',
            padding: 2,
            background: 'transparent',
          }}
        >
          <Box
            sx={{
              background: theme.palette.background.paper,
              borderRadius: 2,
              minHeight: 'calc(100vh - 140px)',
              border: `1px solid ${theme.palette.divider}`,
              overflow: 'hidden',
              padding: 3,
            }}
          >
            <Outlet />
          </Box>
        </Box>
      </Box>
    </Box>
  );
};

export default AppLayout;
