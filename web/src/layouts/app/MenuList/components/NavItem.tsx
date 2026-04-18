import { useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useTheme } from '@mui/material/styles';
import {
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Typography,
} from '@mui/material';
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord';

import { useDispatch, useSelector } from '@/store';
import type { RootState } from '@/store/reducer';
import { activeID, activeItem } from '@/store/slices/menu';
import type { LinkTarget, NavItemType } from '../types';
import { getNavButtonSx, getNavIconSx, NAV_FONT_SIZE } from '../navSx';

interface NavItemProps {
  item: NavItemType;
  level: number;
  parentId?: string;
}

const NavItem = ({ item, level, parentId }: NavItemProps) => {
  const theme = useTheme();
  const dispatch = useDispatch();
  const { pathname } = useLocation();

  const { selectedItem, drawerOpen } = useSelector((state: RootState) => state.menu);
  const isDrawerOpen = typeof drawerOpen === 'boolean' ? drawerOpen : false;
  const isSelected = item.id ? (selectedItem as string[]).includes(item.id) : false;

  const styleParams = { theme, level, isSelected, drawerOpen: isDrawerOpen };

  const IconComponent = item.icon;
  const iconSize = isDrawerOpen ? '18px' : '22px';
  const itemIcon = IconComponent ? (
    <IconComponent
      stroke={1.5}
      size={iconSize}
      style={{
        width: iconSize,
        height: iconSize,
        color: isSelected ? theme.palette.secondary.main : theme.palette.text.primary,
        ...(item.disabled && { opacity: 0.38 }),
      }}
    />
  ) : (
    <FiberManualRecordIcon
      sx={{
        color: isSelected ? theme.palette.secondary.main : theme.palette.text.primary,
        width: isSelected ? 8 : 6,
        height: isSelected ? 8 : 6,
      }}
      fontSize={level > 0 ? 'inherit' : 'medium'}
    />
  );

  const itemTarget: LinkTarget = item.target ? '_blank' : '_self';

  const itemHandler = (id: string) => {
    dispatch(activeItem([id]));
    if (parentId) {
      dispatch(activeID(parentId));
    }
  };

  useEffect(() => {
    if (item.id) {
      const pathSegments = pathname.split('/');
      const currentIndex = pathSegments.findIndex((segment) => segment === item.id);

      if (currentIndex > -1) {
        dispatch(activeItem([item.id]));
      }
    }
  }, [pathname, item.id, dispatch]);

  return (
    <ListItemButton
      component={Link}
      to={item.url || ''}
      target={itemTarget}
      disabled={item.disabled}
      sx={getNavButtonSx(styleParams)}
      selected={isSelected}
      onClick={() => item.id && itemHandler(item.id)}
    >
      <ListItemIcon sx={getNavIconSx(styleParams)}>{itemIcon}</ListItemIcon>

      {(isDrawerOpen || (!isDrawerOpen && level !== 1)) && (
        <ListItemText
          primary={
            <Typography
              sx={{
                fontSize: NAV_FONT_SIZE,
                fontWeight: 500,
                lineHeight: 1.4,
                letterSpacing: '-0.005em',
              }}
              color="inherit"
            >
              {item.title}
            </Typography>
          }
          secondary={
            item.caption && (
              <Typography
                variant="caption"
                sx={{
                  fontSize: '0.6875rem',
                  fontWeight: 400,
                }}
                display="block"
                gutterBottom
              >
                {item.caption}
              </Typography>
            )
          }
        />
      )}
    </ListItemButton>
  );
};

export default NavItem;
