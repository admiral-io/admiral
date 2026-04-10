import React, { type ReactNode, useMemo } from 'react';
import {
  createTheme,
  CssBaseline,
  ThemeProvider as MuiThemeProvider,
  type Theme as MuiTheme,
  useMediaQuery,
  type PaletteMode,
} from '@mui/material';
import { useSelector } from '@/store';

import ComponentStyleOverrides from '@/theme/components';
import Typography from '@/theme/typography';
import Palette from '@/theme/palette';
import { selectThemeMode } from '@/store/slices/user';

interface CustomThemeProviderProps {
  children: ReactNode;
}

const useCustomTheme = (): MuiTheme => {
  const themeMode = useSelector(selectThemeMode);
  const prefersDarkMode = useMediaQuery('(prefers-color-scheme: dark)');

  const paletteMode: PaletteMode =
    themeMode === 'system' ? (prefersDarkMode ? 'dark' : 'light') : themeMode;

  return useMemo(() => {
    try {
      const baseTheme = createTheme({
        palette: Palette(paletteMode),
        // Kill MUI's elevation shadows globally — use borders instead
        shadows: Array(25).fill('none') as unknown as MuiTheme['shadows'],
      });
      return createTheme(baseTheme, {
        typography: Typography(baseTheme),
        components: {
          ...ComponentStyleOverrides(baseTheme),
          // Disable the Material ripple effect globally
          MuiButtonBase: {
            defaultProps: {
              disableRipple: true,
            },
          },
        },
      });
    } catch (error) {
      console.error('Error creating theme:', error);
      return createTheme();
    }
  }, [paletteMode]);
};

const Theme = ({ children }: CustomThemeProviderProps): React.JSX.Element => {
  const customTheme = useCustomTheme();

  return (
    <MuiThemeProvider theme={customTheme}>
      <CssBaseline />
      {children}
    </MuiThemeProvider>
  );
};

export default Theme;