import { memo, useEffect, useMemo } from 'react';
import { useLocation } from 'react-router-dom';
import { Box, Typography } from '@mui/material';
import AppsOutlinedIcon from '@mui/icons-material/AppsOutlined';
import ViewModuleOutlinedIcon from '@mui/icons-material/ViewModuleOutlined';
import FolderCopyOutlinedIcon from '@mui/icons-material/FolderCopyOutlined';
import VpnKeyOutlinedIcon from '@mui/icons-material/VpnKeyOutlined';
import DirectionsRunOutlinedIcon from '@mui/icons-material/DirectionsRunOutlined';
import CloudOutlinedIcon from '@mui/icons-material/CloudOutlined';
import TuneOutlinedIcon from '@mui/icons-material/TuneOutlined';

import NavGroup from './components/NavGroup';
import type { NavItemType } from './types';
import { useDispatch } from '@/store';
import { activeItem } from '@/store/slices/menu';

const menuItems: { items: NavItemType[] } = {
  items: [
    {
      id: 'manage',
      type: 'group',
      children: [
        {
          id: 'applications',
          title: 'Applications',
          type: 'item',
          icon: AppsOutlinedIcon,
          url: '/applications',
        },
        {
          id: 'catalog',
          title: 'Catalog',
          type: 'item',
          icon: ViewModuleOutlinedIcon,
          url: '/catalog',
        },
      ],
    },
    {
      id: 'settings',
      title: 'Settings',
      type: 'group',
      children: [
        {
          id: 'clusters',
          title: 'Clusters',
          type: 'item',
          icon: CloudOutlinedIcon,
          url: '/settings/clusters',
        },
        {
          id: 'runners',
          title: 'Runners',
          type: 'item',
          icon: DirectionsRunOutlinedIcon,
          url: '/settings/runners',
        },
        {
          id: 'credentials',
          title: 'Credentials',
          type: 'item',
          icon: VpnKeyOutlinedIcon,
          url: '/settings/credentials',
        },
        {
          id: 'sources',
          title: 'Repositories',
          type: 'item',
          icon: FolderCopyOutlinedIcon,
          url: '/settings/sources',
        },
        {
          id: 'variables',
          title: 'Variables',
          type: 'item',
          icon: TuneOutlinedIcon,
          url: '/settings/variables',
        },
      ],
    },
  ],
};

function collectNavUrls(items: NavItemType[]): string[] {
  const urls: string[] = [];
  for (const item of items) {
    if (item.url) urls.push(item.url);
    if (item.children) urls.push(...collectNavUrls(item.children));
  }
  return urls;
}

const MenuList = () => {
  const { pathname } = useLocation();
  const dispatch = useDispatch();

  const knownUrls = useMemo(() => collectNavUrls(menuItems.items), []);

  useEffect(() => {
    const matchesSidebarRoute = knownUrls.some((url) => pathname.startsWith(url));
    if (!matchesSidebarRoute) {
      dispatch(activeItem([]));
    }
  }, [pathname, knownUrls, dispatch]);

  const navItems = menuItems.items.map((item, index) => {
    const key = item.id || `menu-item-${index}`;

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
