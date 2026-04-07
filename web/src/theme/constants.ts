// Admiral Design System Constants
// Single source of truth for layout, animation, and component constants

// Layout constants
export const LAYOUT = {
  HEADER_HEIGHT: 64,
  SIDEBAR_WIDTH: 280,
  SIDEBAR_WIDTH_COLLAPSED: 64,
  FOOTER_HEIGHT: 48,
} as const;

// Page layout
export const PAGE_PADDING = {
  sm: 16,
  md: 24,
  lg: 32,
} as const;

// Border radius tokens
export const borderRadius = {
  none: 0,
  xs: 2,
  sm: 4,
  md: 8,
  lg: 12,
  xl: 16,
  full: 9999,
} as const;

// Z-index layers
export const zIndex = {
  background: -1,
  default: 0,
  elevated: 1,
  dropdown: 1000,
  sticky: 1100,
  fixed: 1200,
  modalBackdrop: 1300,
  modal: 1400,
  popover: 1500,
  tooltip: 1600,
  toast: 1700,
} as const;

// Animation durations
export const duration = {
  instant: 0,
  fast: 150,
  normal: 250,
  slow: 350,
  slower: 500,
} as const;

// Animation easings
export const easing = {
  linear: 'linear',
  easeIn: 'cubic-bezier(0.4, 0, 1, 1)',
  easeOut: 'cubic-bezier(0, 0, 0.2, 1)',
  easeInOut: 'cubic-bezier(0.4, 0, 0.2, 1)',
  bounce: 'cubic-bezier(0.68, -0.55, 0.265, 1.55)',
} as const;

// Transitions
export const transitions = {
  // Base transitions
  all: `all ${duration.normal}ms ${easing.easeInOut}`,
  colors: `background-color ${duration.normal}ms ${easing.easeInOut}, color ${duration.normal}ms ${easing.easeInOut}, border-color ${duration.normal}ms ${easing.easeInOut}`,
  opacity: `opacity ${duration.normal}ms ${easing.easeInOut}`,
  transform: `transform ${duration.normal}ms ${easing.easeInOut}`,

  // Component transitions
  button: `all ${duration.fast}ms ${easing.easeOut}`,
  card: `box-shadow ${duration.normal}ms ${easing.easeOut}, transform ${duration.fast}ms ${easing.easeOut}`,
  menu: `opacity ${duration.fast}ms ${easing.easeOut}, transform ${duration.fast}ms ${easing.easeOut}`,
} as const;

// Component sizes
export const iconSize = {
  xs: 16,
  sm: 20,
  md: 24,
  lg: 32,
  xl: 40,
} as const;

export const avatarSize = {
  xs: 24,
  sm: 32,
  md: 40,
  lg: 56,
  xl: 72,
} as const;

// Table constants
export const TABLE = {
  ROW_HEIGHT: 52,
  HEADER_HEIGHT: 56,
  PAGINATION_HEIGHT: 52,
} as const;

// Dialog sizes
export const DIALOG_MAX_WIDTH = {
  xs: 444,
  sm: 600,
  md: 900,
  lg: 1200,
  xl: 1536,
} as const;

// Application constants
export const APP_CONSTANTS = {
  // Loading states
  SKELETON_ANIMATION_DURATION: 1500,
  SPINNER_SIZE: {
    sm: 20,
    md: 40,
    lg: 60,
  },

  // Toast/Snackbar
  TOAST_DURATION: 6000,
  TOAST_MAX_WIDTH: 568,

  // Polling intervals
  POLLING_INTERVAL: {
    fast: 5000, // 5 seconds
    normal: 30000, // 30 seconds
    slow: 60000, // 1 minute
  },

  // Debounce delays
  DEBOUNCE_DELAY: {
    search: 300,
    input: 500,
    resize: 150,
  },

  // Page sizes for pagination
  PAGE_SIZE_OPTIONS: [10, 25, 50, 100],
  DEFAULT_PAGE_SIZE: 25,

  // File size limits
  MAX_FILE_SIZE: {
    image: 5 * 1024 * 1024, // 5MB
    document: 10 * 1024 * 1024, // 10MB
    manifest: 1 * 1024 * 1024, // 1MB
  },

  // Status refresh intervals
  STATUS_REFRESH_INTERVAL: {
    deployment: 5000,
    health: 30000,
    metrics: 60000,
  },
} as const;

// Content and form sizing
export const contentWidth = {
  xs: 320,
  sm: 480,
  md: 640,
  lg: 800,
  xl: 960,
  full: '100%',
} as const;

export const fieldHeight = {
  sm: 32,
  md: 40,
  lg: 48,
} as const;
