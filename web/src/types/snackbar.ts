import type { AlertProps, SnackbarOrigin } from '@mui/material';

export type SnackbarVariant = 'default' | 'alert';
export type SnackbarTransition =
  | 'SlideLeft'
  | 'SlideUp'
  | 'SlideRight'
  | 'SlideDown'
  | 'Grow'
  | 'Fade';
export type SnackbarIconVariant = 'default' | 'hide' | 'useEmojis';

export interface SnackbarState {
  action: boolean;
  open: boolean;
  message: string;
  anchorOrigin: SnackbarOrigin;
  variant: SnackbarVariant;
  alert: AlertProps;
  transition: SnackbarTransition;
  close: boolean;
  dense: boolean;
  maxStack: number;
  iconVariant: SnackbarIconVariant;
  actionButton: boolean;
}
