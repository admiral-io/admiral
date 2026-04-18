import { type JSX, memo, useCallback } from 'react';
import { useDispatch, useSelector } from '@/store';
import {
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  type SxProps,
  Tooltip as MuiTooltip,
  Typography,
} from '@mui/material';
import { alpha, type Theme, useTheme } from '@mui/material/styles';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';

import { openDrawer, selectMenu } from '@/store/slices/menu';
import { NAV_ITEM_SIZE, NAV_FONT_SIZE } from '@/layouts/app/MenuList/navSx';

const SidebarFooter = (): JSX.Element => {
  const theme = useTheme();
  const dispatch = useDispatch();
  const isDark = theme.palette.mode === 'dark';

  const { drawerOpen } = useSelector(selectMenu);

  const handleClick = useCallback((): void => {
    dispatch(openDrawer(!drawerOpen));
  }, [drawerOpen, dispatch]);

  const hoverBg = isDark
    ? alpha(theme.palette.secondary.main, 0.08)
    : theme.palette.secondary.light;

  const listItemButtonSx: SxProps<Theme> = {
    borderRadius: '8px',
    height: NAV_ITEM_SIZE,
    alignItems: 'center',
    justifyContent: drawerOpen ? 'initial' : 'center',
    mb: 0.5,
    pl: drawerOpen ? 1.5 : 1.25,
    transition: 'background-color 150ms ease',
    '&:hover': {
      backgroundColor: drawerOpen ? hoverBg : 'transparent',
    },
  };

  const listItemIconSx: SxProps<Theme> = {
    minWidth: 32,
    borderRadius: '8px',
    alignItems: 'center',
    justifyContent: 'center',
    color: theme.palette.text.secondary,
    transition: 'background-color 150ms ease, color 150ms ease',
    ...(drawerOpen
      ? {}
      : {
          width: NAV_ITEM_SIZE,
          height: NAV_ITEM_SIZE,
          mr: 'auto',
          '&:hover': {
            backgroundColor: hoverBg,
          },
        }),
  };

  const Button = (
    <ListItemButton
      component="div"
      role="button"
      sx={listItemButtonSx}
      aria-expanded={drawerOpen ? 'true' : 'false'}
      aria-label={drawerOpen ? 'Collapse sidebar' : 'Expand sidebar'}
      onClick={handleClick}
    >
      <ListItemIcon sx={listItemIconSx}>
        {drawerOpen ? (
          <ChevronLeftIcon sx={{ fontSize: '18px' }} />
        ) : (
          <ChevronRightIcon sx={{ fontSize: '18px' }} />
        )}
      </ListItemIcon>

      {drawerOpen && (
        <ListItemText
          primary={
            <Typography
              sx={{
                fontSize: NAV_FONT_SIZE,
                fontWeight: 500,
                color: 'text.secondary',
                lineHeight: 1.4,
              }}
            >
              Collapse
            </Typography>
          }
        />
      )}
    </ListItemButton>
  );

  return (
    <List disablePadding sx={{ px: 0.5, pb: 0.5 }}>
      <ListItem
        disablePadding
        sx={{
          display: 'block',
          pt: 1,
          position: 'relative',
          '&::before': {
            content: '""',
            position: 'absolute',
            top: 0,
            left: '10%',
            right: '10%',
            height: '1px',
            background: isDark
              ? `linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.06) 50%, transparent 100%)`
              : `linear-gradient(90deg, transparent 0%, rgba(0,0,0,0.06) 50%, transparent 100%)`,
          },
        }}
      >
        {drawerOpen ? (
          Button
        ) : (
          <MuiTooltip
            title="Expand"
            placement="right"
            enterDelay={500}
            aria-label="Expand sidebar tooltip"
            arrow
          >
            {Button}
          </MuiTooltip>
        )}
      </ListItem>
    </List>
  );
};

export default memo(SidebarFooter);
