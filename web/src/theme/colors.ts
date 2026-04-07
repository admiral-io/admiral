// Design system color tokens
// Single source of truth — replaces theme.module.scss

const colors = {
  // paper & background
  paper: '#ffffff',

  // primary
  primaryLight: '#e0f2f1',
  primaryMain: '#00695c',
  primaryDark: '#004d40',
  primary200: '#80cbc4',
  primary800: '#00251a',

  // secondary
  secondaryLight: '#fff3e0',
  secondaryMain: '#e65100',
  secondaryDark: '#bf360c',
  secondary200: '#ff8a65',
  secondary800: '#bf360c',

  // success
  successLight: '#e8f5e8',
  success200: '#66bb6a',
  successMain: '#388e3c',
  successDark: '#1b5e20',

  // error
  errorLight: '#ef9a9a',
  errorMain: '#d32f2f',
  errorDark: '#b71c1c',

  // warning
  warningLight: '#fffde7',
  warningMain: '#fbc02d',
  warningDark: '#f57f17',

  // info
  infoLight: '#e3f2fd',
  infoMain: '#1976d2',
  infoDark: '#0d47a1',

  // grey
  grey50: '#f8fafc',
  grey100: '#eef2f6',
  grey200: '#e3e8ef',
  grey300: '#cdd5df',
  grey500: '#697586',
  grey600: '#4b5565',
  grey700: '#364152',
  grey900: '#121926',

  // dark theme — paper & background
  darkPaper: '#1a1f2e',
  darkBackground: '#0a0e16',

  // dark levels for elevation
  darkLevel1: '#242b3d',
  darkLevel2: '#2d3449',

  // dark text variants
  darkTextTitle: '#f8fafc',
  darkTextPrimary: '#e2e8f0',
  darkTextSecondary: '#94a3b8',

  // dark primary
  darkPrimaryLight: '#4493f8',
  darkPrimaryMain: '#0969da',
  darkPrimaryDark: '#0550ae',
  darkPrimary200: '#79c0ff',
  darkPrimary800: '#0a3069',

  // dark secondary
  darkSecondaryLight: '#a475f9',
  darkSecondaryMain: '#8250df',
  darkSecondaryDark: '#6639ba',
  darkSecondary200: '#b4a7f8',
  darkSecondary800: '#3e1f79',

  // dark status colors
  darkSuccessLight: '#81c784',
  darkSuccessMain: '#00e676',
  darkSuccessDark: '#00c853',

  darkErrorLight: '#ff5983',
  darkErrorMain: '#ff1744',
  darkErrorDark: '#d50000',

  darkWarningLight: '#ffcc02',
  darkWarningMain: '#ffab00',
  darkWarningDark: '#ff8f00',

  darkInfoLight: '#64b5f6',
  darkInfoMain: '#2979ff',
  darkInfoDark: '#2962ff',
} as const;

export default colors;
