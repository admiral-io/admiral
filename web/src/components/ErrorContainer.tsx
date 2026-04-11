import type React from 'react';
import { Box } from '@mui/material';
import { alpha, useTheme } from '@mui/material/styles';

interface ErrorContainerProps {
  children: React.ReactNode;
}

const ErrorContainer: React.FC<ErrorContainerProps> = ({ children }) => {
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
      {children}
    </Box>
  );
};

export default ErrorContainer;
