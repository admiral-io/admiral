import { memo } from 'react';
import { Box, Typography } from '@mui/material';
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined';
import DeviceHubOutlinedIcon from '@mui/icons-material/DeviceHubOutlined';
import AppsOutlinedIcon from '@mui/icons-material/AppsOutlined';

import { useSelector, type RootState } from '@/store';
import NavGroup from './components/NavGroup';
import type { NavItemType } from './types';

const icons = {
  AppsOutlinedIcon,
  DeviceHubOutlinedIcon,
  SettingsOutlinedIcon,
};

const menuItems: { items: NavItemType[] } = {
  items: [
    {
      id: 'root',
      type: 'group',
      children: [
        {
          id: 'applications',
          title: 'Applications',
          type: 'item',
          icon: icons.AppsOutlinedIcon,
          url: '/applications',
        },
        {
          id: 'clusters',
          title: 'Clusters',
          type: 'item',
          icon: icons.DeviceHubOutlinedIcon,
          url: '/clusters',
        },
        {
          id: 'settings',
          title: 'Settings',
          type: 'collapse',
          icon: icons.SettingsOutlinedIcon,
          children: [
            {
              id: 'users',
              title: 'Users',
              type: 'item',
              url: '/settings/users',
            },
            {
              id: 'variables',
              title: 'Variables',
              type: 'item',
              url: '/settings/variables',
            },
          ],
        },
      ],
    },
  ],
};

const MenuList = () => {
  const { drawerOpen } = useSelector((state: RootState) => state.menu);

  const navItems = menuItems.items.map((item) => {
    const key = item.id || `menu-item-${Math.random()}`;

    switch (item.type) {
      case 'group':
        return <NavGroup key={key} item={item} />;
      default:
        console.warn('Unknown menu item type:', item);
        return (
          <Typography key={key} variant="h6" color="error" align="center">
            Menu Items Error
          </Typography>
        );
    }
  });

  return <Box sx={{ mt: 1.5 }}>{navItems}</Box>;
};

export default memo(MenuList);
