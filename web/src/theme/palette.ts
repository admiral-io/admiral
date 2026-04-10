import type { PaletteMode, PaletteOptions } from '@mui/material';

import { grey, light, dark } from '@/theme/colors';

const Palette = (paletteMode: PaletteMode): PaletteOptions => {
  const tokens = paletteMode === 'dark' ? dark : light;

  return {
    mode: paletteMode,
    common: {
      black: dark.paper,
    },
    primary: {
      light: tokens.primaryLight,
      main: tokens.primaryMain,
      dark: tokens.primaryDark,
      contrastText: '#fff',
    },
    secondary: {
      light: tokens.secondaryLight,
      main: tokens.secondaryMain,
      dark: tokens.secondaryDark,
      contrastText: '#fff',
    },
    error: {
      light: tokens.errorLight,
      main: tokens.errorMain,
      dark: tokens.errorDark,
    },
    warning: {
      light: tokens.warningLight,
      main: tokens.warningMain,
      dark: tokens.warningDark,
    },
    info: {
      light: tokens.infoLight,
      main: tokens.infoMain,
      dark: tokens.infoDark,
    },
    success: {
      light: tokens.successLight,
      main: tokens.successMain,
      dark: tokens.successDark,
    },
    grey: {
      50: grey[50],
      100: grey[100],
      500: tokens.textSecondary,
      600: tokens.textTitle,
      700: tokens.textPrimary,
      900: tokens.textPrimary,
    },
    text: {
      primary: tokens.textPrimary,
      secondary: tokens.textSecondary,
    },
    divider: paletteMode === 'dark' ? grey[700] : grey[200],
    background: {
      paper: tokens.paper,
      default: tokens.background,
    },
  };
};

export default Palette;
