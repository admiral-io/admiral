import React, { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { styled, useTheme } from '@mui/material/styles';
import {
  Box,
  ButtonBase,
  ClickAwayListener,
  Collapse,
  Grow,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Paper,
  Popper,
  Typography,
} from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import ExpandLessIcon from '@mui/icons-material/ExpandLess';
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord';

import NavItem from '../components/NavItem';
import { useSelector } from '@/store';
import type { NavItemType } from '../types';
import { getNavButtonSx, getNavIconSx } from '../styles';

const PopperStyledMini = styled(Popper)(({ theme }) => ({
  overflow: 'visible',
  zIndex: 1202,
  minWidth: 180,
  '&:before': {
    content: '""',
    backgroundColor: theme.palette.background.paper,
    transform: 'translateY(-50%) rotate(45deg)',
    zIndex: 120,
    borderLeft: `1px solid ${theme.palette.divider}`,
    borderBottom: `1px solid ${theme.palette.divider}`,
  },
}));

type VirtualElement = {
  getBoundingClientRect: () => DOMRect;
  contextElement?: Element;
};

interface NavCollapseProps {
  menu: NavItemType;
  level: number;
  parentId: string;
}

const NavCollapse = ({ menu, level, parentId }: NavCollapseProps) => {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const [anchorEl, setAnchorEl] = useState<
    VirtualElement | (() => VirtualElement) | null | undefined
  >(null);
  const { drawerOpen, selectedID } = useSelector((state) => state.menu);

  const handleClickMini = (
    event:
      | React.MouseEvent<HTMLAnchorElement>
      | React.MouseEvent<HTMLDivElement, MouseEvent>
      | undefined,
  ) => {
    setAnchorEl(null);
    if (drawerOpen) {
      setOpen(!open);
    } else {
      setAnchorEl(event?.currentTarget);
    }
  };

  const handleClosePopper = () => {
    setOpen(false);
    setAnchorEl(null);
  };

  const openMini = Boolean(anchorEl);
  const { pathname } = useLocation();

  const checkOpenForParent = (child: NavItemType[]) => {
    child.forEach((item: NavItemType) => {
      if (item.url === pathname) {
        setOpen(true);
      }
    });
  };

  useEffect(() => {
    setOpen(false);
    if (openMini) setAnchorEl(null);
    if (menu.children) {
      menu.children.forEach((item: NavItemType) => {
        if (item.children?.length) {
          checkOpenForParent(item.children);
        }
        if (item.url === pathname) {
          setOpen(true);
        }
      });
    }

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname, menu.children]);

  const menus = menu.children?.map((item) => {
    switch (item.type) {
      case 'collapse':
        return <NavCollapse key={item.id} menu={item} level={level + 1} parentId={parentId} />;
      case 'item':
        return <NavItem key={item.id} item={item} level={level + 1} parentId={parentId} />;
      default:
        return (
          <Typography key={item.id} variant="h6" color="error" align="center">
            Menu Items Error
          </Typography>
        );
    }
  });

  const isSelected = selectedID === menu.id;
  const Icon = menu.icon!;
  const menuIcon = menu.icon ? (
    <Icon
      strokeWidth={1.5}
      size={drawerOpen ? '20px' : '24px'}
      style={{ color: isSelected ? theme.palette.secondary.main : theme.palette.text.primary }}
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

  const renderCollapseIcon = () => {
    if (!drawerOpen) {
      return null;
    }

    if (openMini || open) {
      return (
        <ExpandLessIcon
          sx={{ fontSize: '16px', marginTop: 'auto', marginBottom: 'auto', strokeWidth: 1.5 }}
        />
      );
    }

    return (
      <ExpandMoreIcon
        sx={{ fontSize: '16px', marginTop: 'auto', marginBottom: 'auto', strokeWidth: 1.5 }}
      />
    );
  };

  const styleParams = { theme, level, isSelected, drawerOpen };

  return (
    <>
      <ListItemButton
        sx={{ zIndex: 1201, ...getNavButtonSx(styleParams) } as const}
        selected={isSelected}
        {...(!drawerOpen && { onMouseEnter: handleClickMini, onMouseLeave: handleClosePopper })}
        onClick={handleClickMini}
      >
        {menuIcon && (
          <ButtonBase
            aria-label="theme-icon"
            sx={{ borderRadius: '8px' }}
            disableRipple={drawerOpen}
          >
            <ListItemIcon sx={getNavIconSx(styleParams)}>
              {menuIcon}
            </ListItemIcon>
          </ButtonBase>
        )}
        {(drawerOpen || (!drawerOpen && level !== 1)) && (
          <ListItemText
            primary={
              <Typography variant={isSelected ? 'h5' : 'body1'} color="inherit" sx={{ my: 'auto' }}>
                {menu.title}
              </Typography>
            }
            secondary={
              menu.caption && (
                <Typography
                  variant="caption"
                  sx={{ fontSize: '0.6875rem', fontWeight: 500, textTransform: 'capitalize' }}
                  display="block"
                  gutterBottom
                >
                  {menu.caption}
                </Typography>
              )
            }
          />
        )}

        {renderCollapseIcon()}

        {!drawerOpen && (
          <PopperStyledMini
            open={openMini}
            anchorEl={anchorEl}
            placement="right-start"
            style={{
              zIndex: 2001,
            }}
            modifiers={[
              {
                name: 'offset',
                options: {
                  offset: [-12, 0],
                },
              },
            ]}
          >
            {({ TransitionProps }) => (
              <Grow in={openMini} {...TransitionProps}>
                <Paper
                  sx={{
                    overflow: 'hidden',
                    mt: 1.5,
                    p: 1,
                    boxShadow: theme.shadows[8],
                    backgroundImage: 'none',
                  }}
                >
                  <ClickAwayListener onClickAway={handleClosePopper}>
                    <Box>{menus}</Box>
                  </ClickAwayListener>
                </Paper>
              </Grow>
            )}
          </PopperStyledMini>
        )}
      </ListItemButton>
      {drawerOpen && (
        <Collapse in={open} timeout="auto" unmountOnExit>
          {open && (
            <List
              component="div"
              disablePadding
              sx={{
                position: 'relative',
                '&:after': {
                  content: "''",
                  position: 'absolute',
                  left: '32px',
                  top: 0,
                  height: '100%',
                  width: '1px',
                  opacity: theme.palette.mode === 'dark' ? 0.2 : 1,
                  background:
                    theme.palette.mode === 'dark'
                      ? theme.palette.text.primary
                      : theme.palette.primary.light,
                },
              }}
            >
              {menus}
            </List>
          )}
        </Collapse>
      )}
    </>
  );
};

export default NavCollapse;
