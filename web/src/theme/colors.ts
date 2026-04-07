// Admiral Design System — Color Tokens
// Unified identity: deep navy primary, warm amber/gold accent
// Structured as light/dark token sets with shared neutrals

// ── Shared neutrals ──────────────────────────────────────────
export const grey = {
  50: '#f8fafc',
  100: '#eef2f6',
  200: '#e3e8ef',
  300: '#cdd5df',
  500: '#697586',
  600: '#4b5565',
  700: '#364152',
  900: '#121926',
} as const;

// ── Token shape (both modes export the same keys) ────────────
export interface ModeTokens {
  paper: string;
  background: string;

  primaryLight: string;
  primaryMain: string;
  primaryDark: string;
  primary200: string;
  primary800: string;

  secondaryLight: string;
  secondaryMain: string;
  secondaryDark: string;
  secondary200: string;
  secondary800: string;

  successLight: string;
  successMain: string;
  successDark: string;

  errorLight: string;
  errorMain: string;
  errorDark: string;

  warningLight: string;
  warningMain: string;
  warningDark: string;

  infoLight: string;
  infoMain: string;
  infoDark: string;

  textTitle: string;
  textPrimary: string;
  textSecondary: string;

  // dark-mode-only elevation surfaces (light mode uses paper/background)
  level1: string;
  level2: string;
}

// ── Light mode ───────────────────────────────────────────────
export const light: ModeTokens = {
  paper: '#ffffff',
  background: '#f8fafc',

  // Admiral navy
  primaryLight: '#e8edf5',
  primaryMain: '#1b2a4a',
  primaryDark: '#0f1b33',
  primary200: '#8fa3c7',
  primary800: '#0a1225',

  // Warm amber / gold
  secondaryLight: '#fef7e8',
  secondaryMain: '#d97706',
  secondaryDark: '#92400e',
  secondary200: '#fbbf24',
  secondary800: '#78350f',

  // Status
  successLight: '#e8f5e9',
  successMain: '#388e3c',
  successDark: '#1b5e20',

  errorLight: '#ffebee',
  errorMain: '#d32f2f',
  errorDark: '#b71c1c',

  warningLight: '#fff8e1',
  warningMain: '#f9a825',
  warningDark: '#f57f17',

  infoLight: '#e3f2fd',
  infoMain: '#1976d2',
  infoDark: '#0d47a1',

  // Text
  textTitle: '#121926',
  textPrimary: '#364152',
  textSecondary: '#697586',

  // Surfaces (light mode just uses paper/background)
  level1: '#ffffff',
  level2: '#f8fafc',
} as const;

// ── Dark mode ────────────────────────────────────────────────
export const dark: ModeTokens = {
  paper: '#111827',
  background: '#0b0f19',

  // Admiral navy — lifted for dark backgrounds
  primaryLight: '#93b8f9',
  primaryMain: '#5b8def',
  primaryDark: '#3d6bc7',
  primary200: '#bdd3fc',
  primary800: '#1e3a6e',

  // Warm amber / gold — brightened for contrast
  secondaryLight: '#fde68a',
  secondaryMain: '#f59e0b',
  secondaryDark: '#d97706',
  secondary200: '#fef3c7',
  secondary800: '#92400e',

  // Status — tuned for dark backgrounds
  successLight: '#81c784',
  successMain: '#4caf50',
  successDark: '#2e7d32',

  errorLight: '#ef5350',
  errorMain: '#f44336',
  errorDark: '#c62828',

  warningLight: '#ffb74d',
  warningMain: '#ffa726',
  warningDark: '#e65100',

  infoLight: '#64b5f6',
  infoMain: '#42a5f5',
  infoDark: '#1565c0',

  // Text
  textTitle: '#f1f5f9',
  textPrimary: '#e2e8f0',
  textSecondary: '#94a3b8',

  // Elevation surfaces
  level1: '#1e2536',
  level2: '#262d3f',
} as const;
