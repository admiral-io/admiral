import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

import { type RootState } from '@/store/reducer';
import { type SnackbarIconVariant, type SnackbarState } from '@/types/snackbar';

const initialState: SnackbarState = {
  action: false,
  open: false,
  message: '',
  anchorOrigin: {
    vertical: 'bottom',
    horizontal: 'right',
  },
  variant: 'default',
  alert: {
    color: 'info',
    variant: 'filled',
  },
  transition: 'Fade',
  close: true,
  maxStack: 3,
  dense: false,
  iconVariant: 'hide',
  actionButton: false,
};

const snackbarSlice = createSlice({
  name: 'snackbar',
  initialState,
  reducers: {
    // Opens the snackbar with specified configuration
    openSnackbar(
      state,
      action: PayloadAction<
        Partial<
          Pick<
            SnackbarState,
            | 'open'
            | 'message'
            | 'anchorOrigin'
            | 'variant'
            | 'alert'
            | 'transition'
            | 'close'
            | 'actionButton'
          >
        >
      >,
    ) {
      const { open, message, anchorOrigin, variant, alert, transition, close, actionButton } =
        action.payload;

      state.action = !state.action;
      state.open = open ?? true;
      state.message = message ?? initialState.message;
      state.anchorOrigin = anchorOrigin ?? initialState.anchorOrigin;
      state.variant = variant ?? initialState.variant;
      state.alert = {
        color: alert?.color ?? initialState.alert.color,
        variant: alert?.variant ?? initialState.alert.variant,
      };
      state.transition = transition ?? initialState.transition;
      state.close = close ?? initialState.close;
      state.actionButton = actionButton ?? initialState.actionButton;
    },

    // Closes the snackbar
    closeSnackbar(state) {
      state.open = false;
    },

    // Updates the dense property
    setDenseMode(state, action: PayloadAction<{ dense: boolean }>) {
      state.dense = action.payload.dense;
    },

    // Updates the maximum number of stacked snackbars
    setMaxStack(state, action: PayloadAction<{ maxStack: number }>) {
      state.maxStack = action.payload.maxStack;
    },

    // Updates the icon variant display mode
    setIconVariant(state, action: PayloadAction<{ iconVariant: SnackbarIconVariant }>) {
      state.iconVariant = action.payload.iconVariant;
    },
  },
});

export const snackbar = (state: RootState): SnackbarState => state.snackbar;
export const { openSnackbar, closeSnackbar, setDenseMode, setMaxStack, setIconVariant } =
  snackbarSlice.actions;

export default snackbarSlice.reducer;
