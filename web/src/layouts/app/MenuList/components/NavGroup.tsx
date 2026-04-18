import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { Box, List, Typography } from '@mui/material';

import NavCollapse from '../components/NavCollapse';
import NavItem from '../components/NavItem';
import { useDispatch, useSelector } from '@/store';
import type { NavItemType } from '../types';
import { activeID } from '@/store/slices/menu';

interface NavGroupProps {
  item: NavItemType;
}

const NavGroup = ({ item }: NavGroupProps) => {
  const dispatch = useDispatch();
  const { pathname } = useLocation();
  const { drawerOpen } = useSelector((state) => state.menu);
  const [currentItem] = useState(item);

  const checkOpenForParent = (child: NavItemType[], id: string) => {
    child.forEach((ele: NavItemType) => {
      if (ele.children?.length) {
        checkOpenForParent(ele.children, currentItem.id!);
      }
      if (ele.url === pathname) {
        dispatch(activeID(id));
      }
    });
  };

  const checkSelectedOnload = (data: NavItemType) => {
    const childrens = data.children ? data.children : [];
    childrens.forEach((itemCheck: NavItemType) => {
      if (itemCheck?.children?.length) {
        checkOpenForParent(itemCheck.children, itemCheck.id!);
      }
      if (itemCheck?.url === pathname) {
        dispatch(activeID(currentItem.id!));
      }
    });
  };

  useEffect(() => {
    checkSelectedOnload(currentItem);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname, currentItem]);

  const items = currentItem.children?.map((menu) => {
    switch (menu?.type) {
      case 'collapse':
        return <NavCollapse key={menu.id} menu={menu} level={1} parentId={currentItem.id!} />;
      case 'item':
        return <NavItem key={menu.id} item={menu} level={1} parentId={currentItem.id!} />;
      default:
        return (
          <Typography key={menu?.id} variant="h6" color="error" align="center">
            Menu Items Error
          </Typography>
        );
    }
  });

  const hasTitle = Boolean(currentItem.title);

  return (
    <Box component="nav" sx={{ mt: hasTitle ? 0.5 : 0 }}>
      {hasTitle && (
        <Box
          sx={{
            mx: 1.5,
            pt: 1.5,
            mb: 0.5,
            position: 'relative',
            '&::before': {
              content: '""',
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              height: '1px',
              background: (theme) =>
                theme.palette.mode === 'dark'
                  ? `linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.06) 50%, transparent 100%)`
                  : `linear-gradient(90deg, transparent 0%, rgba(0,0,0,0.06) 50%, transparent 100%)`,
            },
          }}
        >
          {drawerOpen && (
            <Typography
              sx={{
                display: 'block',
                pt: 0.75,
                px: 0.75,
                pb: 0.5,
                fontSize: '0.6875rem',
                fontWeight: 600,
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
                color: 'text.secondary',
                lineHeight: 1,
                userSelect: 'none',
              }}
            >
              {currentItem.title}
            </Typography>
          )}
        </Box>
      )}
      <List disablePadding>{items}</List>
    </Box>
  );
};

export default NavGroup;
