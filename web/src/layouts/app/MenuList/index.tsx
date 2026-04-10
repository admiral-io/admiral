import { memo } from 'react';
import { Box, Typography } from '@mui/material';

import NavGroup from './components/NavGroup';
import type { NavItemType } from './types';

// Sidebar is intentionally empty while the UI is under construction.
// Restore the entries below when shipping features:
//
//   import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined';
//   import DeviceHubOutlinedIcon from '@mui/icons-material/DeviceHubOutlined';
//   import AppsOutlinedIcon from '@mui/icons-material/AppsOutlined';
//
//   const stashedMenuItems: NavItemType[] = [
//     {
//       id: 'applications',
//       title: 'Applications',
//       type: 'item',
//       icon: AppsOutlinedIcon,
//       url: '/applications',
//     },
//     {
//       id: 'clusters',
//       title: 'Clusters',
//       type: 'item',
//       icon: DeviceHubOutlinedIcon,
//       url: '/clusters',
//     },
//     {
//       id: 'settings',
//       title: 'Settings',
//       type: 'collapse',
//       icon: SettingsOutlinedIcon,
//       children: [
//         { id: 'users', title: 'Users', type: 'item', url: '/settings/users' },
//         { id: 'variables', title: 'Variables', type: 'item', url: '/settings/variables' },
//       ],
//     },
//   ];

const menuItems: { items: NavItemType[] } = {
  items: [
    {
      id: 'root',
      type: 'group',
      children: [],
    },
  ],
};

const MenuList = () => {
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
