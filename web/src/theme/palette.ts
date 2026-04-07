import type { PaletteMode, PaletteOptions } from '@mui/material';

import colors from '@/theme/colors';

const Palette = (paletteMode: PaletteMode): PaletteOptions => ({
  mode: paletteMode,
  common: {
    black: colors.darkPaper,
  },
  primary: {
    light: paletteMode === 'dark' ? colors.darkPrimaryLight : colors.primaryLight,
    main: paletteMode === 'dark' ? colors.darkPrimaryMain : colors.primaryMain,
    dark: paletteMode === 'dark' ? colors.darkPrimaryDark : colors.primaryDark,
    contrastText: '#fff',
  },
  secondary: {
    light: paletteMode === 'dark' ? colors.darkSecondaryLight : colors.secondaryLight,
    main: paletteMode === 'dark' ? colors.darkSecondaryMain : colors.secondaryMain,
    dark: paletteMode === 'dark' ? colors.darkSecondaryDark : colors.secondaryDark,
    contrastText: '#fff',
  },
  error: {
    light: paletteMode === 'dark' ? colors.darkErrorLight : colors.errorLight,
    main: paletteMode === 'dark' ? colors.darkErrorMain : colors.errorMain,
    dark: paletteMode === 'dark' ? colors.darkErrorDark : colors.errorDark,
  },
  warning: {
    light: paletteMode === 'dark' ? colors.darkWarningLight : colors.warningLight,
    main: paletteMode === 'dark' ? colors.darkWarningMain : colors.warningMain,
    dark: paletteMode === 'dark' ? colors.darkWarningDark : colors.warningDark,
  },
  info: {
    light: paletteMode === 'dark' ? colors.darkInfoLight : colors.infoLight,
    main: paletteMode === 'dark' ? colors.darkInfoMain : colors.infoMain,
    dark: paletteMode === 'dark' ? colors.darkInfoDark : colors.infoDark,
  },
  success: {
    light: paletteMode === 'dark' ? colors.darkSuccessLight : colors.successLight,
    main: paletteMode === 'dark' ? colors.darkSuccessMain : colors.successMain,
    dark: paletteMode === 'dark' ? colors.darkSuccessDark : colors.successDark,
  },
  grey: {
    50: colors.grey50,
    100: colors.grey100,
    500: paletteMode === 'dark' ? colors.darkTextSecondary : colors.grey500,
    600: paletteMode === 'dark' ? colors.darkTextTitle : colors.grey600,
    700: paletteMode === 'dark' ? colors.darkTextPrimary : colors.grey700,
    900: paletteMode === 'dark' ? colors.darkTextPrimary : colors.grey900,
  },
  text: {
    primary: paletteMode === 'dark' ? colors.darkTextPrimary : colors.grey700,
    secondary: paletteMode === 'dark' ? colors.darkTextSecondary : colors.grey500,
  },
  divider: paletteMode === 'dark' ? colors.grey700 : colors.grey200,
  background: {
    paper: paletteMode === 'dark' ? colors.darkPaper : colors.paper,
    default: paletteMode === 'dark' ? colors.darkBackground : colors.paper,
  },
});

export default Palette;
