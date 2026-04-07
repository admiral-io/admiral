import React, { useState } from 'react';
import {
  Box,
  Avatar,
  Menu,
  IconButton,
  Typography,
  ToggleButtonGroup,
  ToggleButton,
  Stack,
  useTheme,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import DarkModeIcon from '@mui/icons-material/DarkMode';
import LightModeIcon from '@mui/icons-material/LightMode';
import SettingsBrightnessIcon from '@mui/icons-material/SettingsBrightness';
import { useSelector, useDispatch } from 'react-redux';

import type { RootState } from '@/store';
import { type ThemeMode, setThemeMode } from '@/store/slices/user';
import { getValidPictureUrl, getAvatarInitial } from '@/utils/avatar';

const ThemeSelector = ({
  value,
  onChange,
}: {
  value: ThemeMode;
  onChange: (event: React.MouseEvent<HTMLElement>, value: ThemeMode | null) => void;
}) => (
  <Box sx={{ px: 1, py: 0.5 }}>
    <Typography variant="subtitle1" sx={{ mb: 1, fontWeight: 'bold' }}>
      Theme
    </Typography>
    <ToggleButtonGroup
      value={value}
      exclusive
      onChange={onChange}
      aria-label="theme mode"
      size="small"
      fullWidth
      sx={{ mb: 1 }}
    >
      <ToggleButton value="light" aria-label="light mode">
        <Stack direction="row" spacing={1} alignItems="center">
          <LightModeIcon fontSize="small" />
          <Typography variant="body2">Light</Typography>
        </Stack>
      </ToggleButton>
      <ToggleButton value="dark" aria-label="dark mode">
        <Stack direction="row" spacing={1} alignItems="center">
          <DarkModeIcon fontSize="small" />
          <Typography variant="body2">Dark</Typography>
        </Stack>
      </ToggleButton>
      <ToggleButton value="system" aria-label="system mode">
        <Stack direction="row" spacing={1} alignItems="center">
          <SettingsBrightnessIcon fontSize="small" />
          <Typography variant="body2">Auto</Typography>
        </Stack>
      </ToggleButton>
    </ToggleButtonGroup>
  </Box>
);

const Header: React.FC = () => {
  const theme = useTheme();
  const dispatch = useDispatch();
  const { display_name, avatar_url, email } = useSelector((s: RootState) => s.user);
  const themeMode = useSelector((s: RootState) => s.user.preferences.themeMode);
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);

  const handleMenuOpen = (e: React.MouseEvent<HTMLElement>) => setAnchorEl(e.currentTarget);
  const handleMenuClose = () => setAnchorEl(null);

  const handleThemeChange = (_: React.MouseEvent<HTMLElement>, newTheme: ThemeMode | null) => {
    if (newTheme !== null) {
      dispatch(setThemeMode(newTheme));
    }
  };

  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        p: 2,
        borderBottom: 1,
        borderColor: 'divider',
        backdropFilter: 'blur(10px)',
        background:
          theme.palette.mode === 'dark'
            ? alpha(theme.palette.background.default, 0.9)
            : alpha(theme.palette.background.paper, 0.8),
        position: 'sticky',
        top: 0,
        zIndex: 1100,
      }}
    >
      <Box />
      <IconButton
        onClick={handleMenuOpen}
        size="small"
        sx={{
          transition: 'all 0.2s ease-in-out',
          '&:hover': {
            transform: 'scale(1.05)',
          },
          p: 1, // Use equal padding on all sides for circular shape
          borderRadius: '50%', // Ensure the hover effect is circular
        }}
      >
        <Avatar
          alt={display_name || email}
          src={getValidPictureUrl(avatar_url)}
          sx={{
            width: 36,
            height: 36,
            boxShadow:
              theme.palette.mode === 'dark'
                ? `0px 4px 12px ${alpha('#000', 0.4)}, 0px 0px 0px 2px ${alpha('#fff', 0.1)}`
                : `0px 4px 12px ${alpha('#000', 0.15)}, 0px 0px 0px 2px ${theme.palette.background.paper}`,
            border: `2px solid ${theme.palette.background.paper}`,
            transition: 'all 0.2s ease-in-out',
          }}
        >
          {getAvatarInitial(display_name, email)}
        </Avatar>
      </IconButton>
      <Menu
        anchorEl={anchorEl}
        open={open}
        onClose={handleMenuClose}
        slotProps={{
          paper: {
            elevation: 4,
            sx: {
              borderRadius: 2,
              minWidth: 220,
              backgroundColor: 'background.paper',
              p: 1,
            },
          },
        }}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        disableAutoFocusItem
      >
        <ThemeSelector value={themeMode} onChange={handleThemeChange} />
      </Menu>
    </Box>
  );
};

export default Header;
