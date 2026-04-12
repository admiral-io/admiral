import React, { useState } from 'react';
import {
  Box,
  Avatar,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
  useTheme,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import DarkModeIcon from '@mui/icons-material/DarkMode';
import KeyOutlinedIcon from '@mui/icons-material/KeyOutlined';
import LightModeIcon from '@mui/icons-material/LightMode';
import PersonOutlineIcon from '@mui/icons-material/PersonOutline';
import SettingsBrightnessIcon from '@mui/icons-material/SettingsBrightness';
import { useSelector, useDispatch } from 'react-redux';
import { useNavigate } from 'react-router-dom';

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
    <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: 'block' }}>
      Theme
    </Typography>
    <ToggleButtonGroup
      value={value}
      exclusive
      onChange={onChange}
      aria-label="theme mode"
      size="small"
      fullWidth
    >
      <ToggleButton value="light" aria-label="light mode">
        <Stack direction="row" spacing={0.5} alignItems="center">
          <LightModeIcon sx={{ fontSize: 16 }} />
          <Typography variant="caption">Light</Typography>
        </Stack>
      </ToggleButton>
      <ToggleButton value="dark" aria-label="dark mode">
        <Stack direction="row" spacing={0.5} alignItems="center">
          <DarkModeIcon sx={{ fontSize: 16 }} />
          <Typography variant="caption">Dark</Typography>
        </Stack>
      </ToggleButton>
      <ToggleButton value="system" aria-label="system mode">
        <Stack direction="row" spacing={0.5} alignItems="center">
          <SettingsBrightnessIcon sx={{ fontSize: 16 }} />
          <Typography variant="caption">Auto</Typography>
        </Stack>
      </ToggleButton>
    </ToggleButtonGroup>
  </Box>
);

const Header: React.FC = () => {
  const theme = useTheme();
  const dispatch = useDispatch();
  const navigate = useNavigate();
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

  const navigateTo = (path: string) => {
    handleMenuClose();
    navigate(path);
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
        aria-label="Account menu"
        sx={{
          transition: 'all 0.2s ease-in-out',
          '&:hover': { transform: 'scale(1.05)' },
          p: 1,
          borderRadius: '50%',
        }}
      >
        <Avatar
          alt={display_name || email}
          src={getValidPictureUrl(avatar_url)}
          sx={{
            width: 36,
            height: 36,
            boxShadow: 'none',
            border: `2px solid ${theme.palette.divider}`,
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
            elevation: 0,
            sx: {
              borderRadius: 1.5,
              minWidth: 260,
              backgroundColor: 'background.paper',
              border: `1px solid ${theme.palette.divider}`,
              p: 0,
              overflow: 'visible',
              mt: 0.5,
            },
          },
        }}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        disableAutoFocusItem
      >
        <Box sx={{ px: 2, py: 1.5 }}>
          <Stack direction="row" spacing={1.5} alignItems="center">
            <Avatar
              alt={display_name || email}
              src={getValidPictureUrl(avatar_url)}
              sx={{ width: 40, height: 40 }}
            >
              {getAvatarInitial(display_name, email)}
            </Avatar>
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="subtitle2" noWrap>
                {display_name || 'User'}
              </Typography>
              <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
                {email}
              </Typography>
            </Box>
          </Stack>
        </Box>

        <Divider />

        <Box sx={{ py: 0.5 }}>
          <MenuItem onClick={() => navigateTo('/user/profile')}>
            <ListItemIcon><PersonOutlineIcon fontSize="small" /></ListItemIcon>
            <ListItemText>Profile</ListItemText>
          </MenuItem>
          <MenuItem onClick={() => navigateTo('/user/tokens')}>
            <ListItemIcon><KeyOutlinedIcon fontSize="small" /></ListItemIcon>
            <ListItemText>Personal Access Tokens</ListItemText>
          </MenuItem>
        </Box>

        <Divider />

        <Box sx={{ p: 1 }}>
          <ThemeSelector value={themeMode} onChange={handleThemeChange} />
        </Box>

      </Menu>
    </Box>
  );
};

export default Header;
