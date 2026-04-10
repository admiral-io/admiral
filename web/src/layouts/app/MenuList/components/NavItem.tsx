import { useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useTheme } from '@mui/material/styles';
import {
  Avatar,
  ButtonBase,
  Chip,
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
import { getNavButtonSx, getNavIconSx } from '../styles';

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
  const itemIcon = IconComponent ? (
    <IconComponent
      stroke={1.5}
      size={isDrawerOpen ? '20px' : '24px'}
      style={{
        color: isSelected ? theme.palette.secondary.main : theme.palette.text.primary,
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
      <ButtonBase aria-label="theme-icon" sx={{ borderRadius: '8px' }}>
        <ListItemIcon sx={getNavIconSx(styleParams)}>{itemIcon}</ListItemIcon>
      </ButtonBase>

      {(isDrawerOpen || (!isDrawerOpen && level !== 1)) && (
        <ListItemText
          primary={
            <Typography variant={isSelected ? 'h5' : 'body1'} color="inherit">
              {item.title}
            </Typography>
          }
          secondary={
            item.caption && (
              <Typography
                variant="caption"
                sx={{ fontSize: '0.6875rem', fontWeight: 500, textTransform: 'capitalize' }}
                display="block"
                gutterBottom
              >
                {item.caption}
              </Typography>
            )
          }
        />
      )}

      {isDrawerOpen && item.chip && (
        <Chip
          color={item.chip.color}
          variant={item.chip.variant}
          size={item.chip.size}
          label={item.chip.label}
          avatar={item.chip.avatar ? <Avatar>{item.chip.avatar}</Avatar> : undefined}
        />
      )}
    </ListItemButton>
  );
};

export default NavItem;