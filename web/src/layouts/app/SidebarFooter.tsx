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
  ButtonBase,
  Typography,
} from '@mui/material';
import { alpha, type Theme, useTheme } from '@mui/material/styles';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';

import { openDrawer, selectMenu } from '@/store/slices/menu';

const SidebarFooter = (): JSX.Element => {
  const theme = useTheme();
  const dispatch = useDispatch();

  const { drawerOpen } = useSelector(selectMenu);
  const isDrawerOpen = drawerOpen;

  const handleClick = useCallback((): void => {
    dispatch(openDrawer(!drawerOpen));
  }, [drawerOpen, dispatch]);

  const listItemButtonSx: SxProps<Theme> = {
    borderRadius: '8px',
    height: 46,
    alignItems: 'center',
    justifyContent: drawerOpen ? 'initial' : 'center',
    mb: 0.5,
    pl: drawerOpen ? 1.5 : 1.25,
    ...(drawerOpen && theme.palette.mode !== 'dark'
      ? {
          '&:hover': {
            backgroundColor: theme.palette.secondary.light,
          },
        }
      : {
          '&:hover': {
            backgroundColor: 'transparent',
          },
          '&.Mui-selected': {
            backgroundColor: 'transparent',
            '&:hover': {
              backgroundColor: 'transparent',
            },
          },
        }),
  };

  const listItemIconSx: SxProps<Theme> = {
    borderRadius: '8px',
    alignItems: 'center',
    justifyContent: 'center',
    color: 'inherit',
    ...(drawerOpen
      ? {
          mr: 1,
          '&:hover': {
            backgroundColor: 'transparent',
          },
          '&.Mui-selected': {
            '&:hover': {
              backgroundColor: 'transparent',
            },
            backgroundColor: 'transparent',
          },
        }
      : {
          width: 46,
          height: 46,
          mr: 'auto',
          '&:hover': {
            backgroundColor:
              theme.palette.mode === 'dark'
                ? alpha(theme.palette.secondary.main, 0.25)
                : theme.palette.secondary.light,
          },
          '&.Mui-selected': {
            backgroundColor: theme.palette.secondary.light,
            '&:hover': {
              backgroundColor: theme.palette.secondary.light,
            },
          },
        }),
  };

  const Button = (
    <ListItemButton
      component="div"
      role="button"
      sx={listItemButtonSx}
      aria-expanded={drawerOpen ? 'true' : 'false'}
      aria-label={drawerOpen ? 'Collapse drawer' : 'Expand drawer'}
      onClick={handleClick}
    >
      <ButtonBase aria-label="theme-icon" sx={{ borderRadius: '8px' }}>
        <ListItemIcon sx={listItemIconSx}>
          {drawerOpen ? <ChevronLeftIcon /> : <ChevronRightIcon />}
        </ListItemIcon>
      </ButtonBase>

      {isDrawerOpen && (
        <ListItemText
          primary={
            <Typography variant="body1" color="inherit">
              Collapse
            </Typography>
          }
        />
      )}
    </ListItemButton>
  );

  return (
    <List>
      <ListItem
        disablePadding
        sx={{
          display: 'block',
          pt: '12px',
          position: 'relative',
          '&::before': {
            content: '""',
            position: 'absolute',
            top: 0,
            left: '10%',
            right: '10%',
            height: '1px',
            background: `linear-gradient(90deg, transparent 0%, rgba(136, 136, 136, 0.2) 50%, transparent 100%)`,
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
            aria-label="Expand drawer tooltip"
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
