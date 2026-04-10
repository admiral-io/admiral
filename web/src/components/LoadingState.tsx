import React from 'react';
import { Box, CircularProgress, Typography, LinearProgress } from '@mui/material';

export interface LoadingStateProps {
  variant?: 'circular' | 'linear' | 'skeleton';
  message?: string;
  size?: 'small' | 'medium' | 'large';
  fullHeight?: boolean;
  overlay?: boolean;
  progress?: number;
  sx?: object;
}

const sizeMap = {
  small: 24,
  medium: 40,
  large: 60,
};

export const LoadingState: React.FC<LoadingStateProps> = ({
  variant = 'circular',
  message,
  size = 'medium',
  fullHeight = true,
  overlay = false,
  progress,
  sx = {},
}) => {
  const content = (
    <>
      {variant === 'circular' && (
        <CircularProgress
          size={sizeMap[size]}
          thickness={size === 'small' ? 5 : 4}
          variant={progress !== undefined ? 'determinate' : 'indeterminate'}
          value={progress}
        />
      )}

      {variant === 'linear' && (
        <Box sx={{ width: '100%', maxWidth: 400 }}>
          <LinearProgress
            variant={progress !== undefined ? 'determinate' : 'indeterminate'}
            value={progress}
          />
        </Box>
      )}

      {message && (
        <Typography
          variant={size === 'small' ? 'body2' : 'body1'}
          sx={{
            mt: 2,
            color: 'text.secondary',
            textAlign: 'center',
          }}
        >
          {message}
        </Typography>
      )}
    </>
  );

  if (overlay) {
    return (
      <Box
        sx={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: 'rgba(255, 255, 255, 0.9)',
          backdropFilter: 'blur(2px)',
          zIndex: 1,
          ...sx,
        }}
      >
        {content}
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: fullHeight ? 400 : 'auto',
        p: 4,
        ...sx,
      }}
    >
      {content}
    </Box>
  );
};
