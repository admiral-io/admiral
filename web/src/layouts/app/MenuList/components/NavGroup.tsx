import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useTheme } from '@mui/material/styles';
import { List, Typography } from '@mui/material';

import NavCollapse from '../components/NavCollapse';
import NavItem from '../components/NavItem';
import { useDispatch, useSelector } from '@/store';
import type { NavItemType } from '../types';
import { activeID } from '@/store/slices/menu';

type VirtualElement = {
  getBoundingClientRect: () => DOMRect;
  contextElement?: Element;
};

interface NavGroupProps {
  item: NavItemType;
}

const NavGroup = ({ item }: NavGroupProps) => {
  const theme = useTheme();
  const dispatch = useDispatch();
  const { pathname } = useLocation();
  const { drawerOpen } = useSelector((state) => state.menu);
  const [anchorEl, setAnchorEl] = useState<
    VirtualElement | (() => VirtualElement) | null | undefined
  >(null);
  const [currentItem] = useState(item);
  const openMini = Boolean(anchorEl);

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

  // keep selected-menu on page load and use for horizontal menu close on change routes
  useEffect(() => {
    checkSelectedOnload(currentItem);
    if (openMini) setAnchorEl(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname, currentItem]);

  // menu list collapse & items
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

  return (
    <>
      <List
        disablePadding
        subheader={
          currentItem.title && drawerOpen ? (
            <Typography
              variant="caption"
              sx={{
                fontSize: '0.875rem',
                fontWeight: 500,
                color: theme.palette.mode === 'dark' ? 'grey.600' : 'grey.900',
                p: '6px',
                textTransform: 'capitalize',
                mt: '10px',
              }}
              display="block"
              gutterBottom
            >
              {currentItem.title}
              {currentItem.caption && (
                <Typography
                  variant="caption"
                  sx={{ fontSize: '0.6875rem', fontWeight: 500, textTransform: 'capitalize' }}
                  display="block"
                  gutterBottom
                >
                  {currentItem.caption}
                </Typography>
              )}
            </Typography>
          ) : undefined
        }
      >
        {items}
      </List>
    </>
  );
};

export default NavGroup;
