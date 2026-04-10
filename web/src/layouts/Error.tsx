import React from 'react';
import { Box } from '@mui/material';
import { alpha, useTheme } from '@mui/material/styles';
import { Outlet } from 'react-router-dom';

const ErrorLayout: React.FC = () => {
  const theme = useTheme();
  const isDark = theme.palette.mode === 'dark';

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: '100vh',
        px: 3,
        background: isDark
          ? `linear-gradient(135deg, ${alpha(theme.palette.primary.dark, 0.15)} 0%, ${theme.palette.background.default} 50%)`
          : `linear-gradient(135deg, ${alpha(theme.palette.primary.light, 0.4)} 0%, ${theme.palette.background.default} 50%)`,
      }}
    >
      <Outlet />
    </Box>
  );
};

export default ErrorLayout;