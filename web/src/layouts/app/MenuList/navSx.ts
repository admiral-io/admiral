import type { Theme } from '@mui/material/styles';
import { alpha } from '@mui/material/styles';
import type { SxProps } from '@mui/material';

interface NavStyleParams {
  theme: Theme;
  level: number;
  isSelected: boolean;
  drawerOpen: boolean;
}

const NAV_ITEM_SIZE = 38;
const NAV_FONT_SIZE = '0.8125rem';

function hoverBackground(theme: Theme, drawerOpen: boolean, level: number): string {
  if (!drawerOpen && level === 1) return 'transparent';

  return theme.palette.mode === 'dark'
    ? alpha(theme.palette.secondary.main, 0.08)
    : theme.palette.secondary.light;
}

function selectedColor(theme: Theme, drawerOpen: boolean): string {
  return theme.palette.mode === 'dark' && drawerOpen
    ? theme.palette.text.primary
    : theme.palette.secondary.main;
}

export function getNavButtonSx({
  theme,
  level,
  drawerOpen,
}: Pick<NavStyleParams, 'theme' | 'level' | 'drawerOpen'>): SxProps<Theme> {
  const hover = hoverBackground(theme, drawerOpen, level);
  const selected = selectedColor(theme, drawerOpen);

  return {
    borderRadius: '8px',
    mb: '2px',
    pl: drawerOpen ? `${level * 20}px` : 1.25,
    height: level === 1 ? NAV_ITEM_SIZE : 'auto',
    py: level === 1 ? 0 : 0.75,
    transition: 'background-color 150ms ease, color 150ms ease',
    '&:hover': {
      backgroundColor: hover,
    },
    '&.Mui-selected': {
      backgroundColor: drawerOpen ? hover : 'transparent',
      color: selected,
      '&:hover': {
        backgroundColor: hover,
        color: selected,
      },
    },
  };
}

export function getNavIconSx({
  theme,
  level,
  isSelected,
  drawerOpen,
}: NavStyleParams): SxProps<Theme> {
  const isDark = theme.palette.mode === 'dark';
  const selected = selectedColor(theme, drawerOpen);
  const text =
    isDark ? theme.palette.grey[400] : theme.palette.text.primary;

  const base: SxProps<Theme> = {
    minWidth: level === 1 ? 32 : 18,
    color: isSelected ? selected : text,
    transition: 'color 150ms ease',
  };

  if (!drawerOpen && level === 1) {
    return {
      ...base,
      borderRadius: '8px',
      width: NAV_ITEM_SIZE,
      height: NAV_ITEM_SIZE,
      alignItems: 'center',
      justifyContent: 'center',
      transition: 'background-color 150ms ease, color 150ms ease',
      '&:hover': {
        backgroundColor: isDark
          ? alpha(theme.palette.secondary.main, 0.15)
          : theme.palette.secondary.light,
      },
      ...(isSelected && {
        backgroundColor: isDark
          ? alpha(theme.palette.secondary.main, 0.15)
          : theme.palette.secondary.light,
        '&:hover': {
          backgroundColor: isDark
            ? alpha(theme.palette.secondary.main, 0.2)
            : theme.palette.secondary.light,
        },
      }),
    };
  }

  return base;
}

export { NAV_ITEM_SIZE, NAV_FONT_SIZE };
