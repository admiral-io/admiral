import type { Theme } from '@mui/material/styles';
import { alpha } from '@mui/material/styles';
import type { SxProps } from '@mui/material';

interface NavStyleParams {
  theme: Theme;
  level: number;
  isSelected: boolean;
  drawerOpen: boolean;
}

const textColor = (theme: Theme) =>
  theme.palette.mode === 'dark' ? 'grey.400' : 'text.primary';

const iconSelectedColor = (theme: Theme, drawerOpen: boolean) =>
  theme.palette.mode === 'dark' && drawerOpen ? 'text.primary' : 'secondary.main';

export function getNavButtonSx({ theme, level, isSelected, drawerOpen }: NavStyleParams): SxProps<Theme> {
  const base: SxProps<Theme> = {
    borderRadius: '8px',
    mb: 0.5,
    pl: drawerOpen ? `${level * 24}px` : 1.25,
    ...(level === 1 && { height: 46 }),
  };

  if (drawerOpen && level === 1 && theme.palette.mode !== 'dark') {
    return {
      ...base,
      '&:hover': {
        backgroundColor: theme.palette.secondary.light,
      },
      '&.Mui-selected': {
        backgroundColor: theme.palette.secondary.light,
        color: iconSelectedColor(theme, drawerOpen),
        '&:hover': {
          color: iconSelectedColor(theme, drawerOpen),
          backgroundColor: theme.palette.secondary.light,
        },
      },
    };
  }

  if (!drawerOpen || level !== 1) {
    const submenuHoverColor =
      level !== 1
        ? theme.palette.mode === 'dark'
          ? alpha(theme.palette.secondary.main, 0.15)
          : theme.palette.secondary.light
        : 'transparent';

    return {
      ...base,
      py: level === 1 ? 0 : 1,
      '&:hover': {
        backgroundColor: level === 1 && !drawerOpen ? 'transparent' : submenuHoverColor,
      },
      '&.Mui-selected': {
        backgroundColor: 'transparent',
        '&:hover': {
          backgroundColor: level === 1 && !drawerOpen ? 'transparent' : submenuHoverColor,
        },
      },
    };
  }

  return base;
}

export function getNavIconSx({ theme, level, isSelected, drawerOpen }: NavStyleParams): SxProps<Theme> {
  const isDark = theme.palette.mode === 'dark';
  const selected = iconSelectedColor(theme, drawerOpen);
  const text = textColor(theme);

  const base: SxProps<Theme> = {
    minWidth: level === 1 ? 36 : 18,
    color: isSelected ? selected : text,
  };

  if (!drawerOpen && level === 1) {
    return {
      ...base,
      borderRadius: '8px',
      width: 46,
      height: 46,
      alignItems: 'center',
      justifyContent: 'center',
      '&:hover': {
        backgroundColor: isDark ? alpha(theme.palette.secondary.main, 0.25) : 'secondary.light',
      },
      ...(isSelected && {
        backgroundColor: isDark ? alpha(theme.palette.secondary.main, 0.25) : 'secondary.light',
        '&:hover': {
          backgroundColor: isDark ? alpha(theme.palette.secondary.main, 0.3) : 'secondary.light',
        },
      }),
    };
  }

  return base;
}

export { textColor, iconSelectedColor };