import type { ReactElement, HTMLAttributes } from 'react';
import { LinearProgress, Fade, styled } from '@mui/material';
import type { LinearProgressProps } from '@mui/material';

interface LoaderProps extends HTMLAttributes<HTMLDivElement> {
  color?: LinearProgressProps['color'];
  visible?: boolean;
  position?: 'top' | 'bottom' | 'inline';
  height?: number | string;
  ariaLabel?: string;
}

const LoaderWrapper = styled('div', {
  shouldForwardProp: (prop) => !['position', 'height'].includes(prop as string),
})<{ position: LoaderProps['position']; height: LoaderProps['height'] }>(
  ({ theme, position, height }) => ({
    position: position === 'inline' ? 'relative' : 'fixed',
    top: position === 'top' ? 0 : 'auto',
    bottom: position === 'bottom' ? 0 : 'auto',
    left: 0,
    width: '100%',
    zIndex: position === 'inline' ? 'auto' : theme.zIndex.modal + 1,
    height: height ?? 4,
    backgroundColor: 'transparent',
    '& .MuiLinearProgress-root': {
      height: '100%',
      backgroundColor:
        theme.palette.mode === 'light' ? theme.palette.grey[200] : theme.palette.grey[800],
    },
  }),
);

const Loader = ({
  color = 'primary',
  visible = true,
  position = 'top',
  height,
  ariaLabel = 'Loading',
  ...rest
}: LoaderProps): ReactElement => (
  <Fade in={visible} timeout={{ enter: 300, exit: 200 }} unmountOnExit>
    <LoaderWrapper position={position} height={height} {...rest}>
      <LinearProgress color={color} role="progressbar" aria-label={ariaLabel} />
    </LoaderWrapper>
  </Fade>
);

export default Loader;
