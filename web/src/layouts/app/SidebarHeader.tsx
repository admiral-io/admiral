import type { JSX } from 'react';
import { Box, type BoxProps } from '@mui/material';
import { useSelector } from 'react-redux';

import { selectMenu } from '@/store/slices/menu';
import { Logo } from '@/components/Logo';

const SidebarHeader = (): JSX.Element => {
  const { drawerOpen } = useSelector(selectMenu);

  const boxProps: BoxProps = {
    sx: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 2,
      minHeight: 72,
      position: 'relative',
      '&::after': {
        content: '""',
        position: 'absolute',
        bottom: 0,
        left: '10%',
        right: '10%',
        height: '1px',
        background: `linear-gradient(90deg, transparent 0%, rgba(136, 136, 136, 0.2) 50%, transparent 100%)`,
      },
    },
  };

  return (
    <Box {...boxProps}>
      <Logo width={drawerOpen ? 90 : 25} height={25} />
    </Box>
  );
};

export default SidebarHeader;
