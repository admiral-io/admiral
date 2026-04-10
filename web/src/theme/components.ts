import { alpha, type Theme } from '@mui/material/styles';
import { borderRadius, transitions } from '@/theme/constants';

const ComponentStyleOverrides = (theme: Theme) => {
  const isDark = theme.palette.mode === 'dark';
  const menuSelectedBack =
    isDark ? alpha(theme.palette.secondary.main, 0.08) : theme.palette.secondary.light;
  const menuSelected = theme.palette.secondary.main;

  return {
    MuiCssBaseline: {
      styleOverrides: {
        '::-webkit-scrollbar': {
          width: '8px',
          height: '8px',
        },
        '::-webkit-scrollbar-track': {
          background: 'transparent',
        },
        '::-webkit-scrollbar-thumb': {
          background: isDark ? alpha('#fff', 0.2) : alpha('#000', 0.2),
          borderRadius: `${borderRadius.sm}px`,
        },
        '::-webkit-scrollbar-thumb:hover': {
          background: isDark ? alpha('#fff', 0.3) : alpha('#000', 0.3),
        },
        'button:focus-visible, input:focus-visible, select:focus-visible, textarea:focus-visible': {
          outline: `2px solid ${theme.palette.primary.main}`,
          outlineOffset: '2px',
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          fontWeight: 600,
          borderRadius: `${borderRadius.md}px`,
          textTransform: 'none' as const,
          boxShadow: 'none',
          '&.MuiButton-sizeSmall': {
            padding: '6px 12px',
            fontSize: '0.8125rem',
            minHeight: '32px',
          },
          '&.MuiButton-sizeMedium': {
            padding: '8px 16px',
            fontSize: '0.875rem',
            minHeight: '36px',
          },
          '&.MuiButton-sizeLarge': {
            padding: '11px 22px',
            fontSize: '0.9375rem',
            minHeight: '42px',
          },
          '&.Mui-disabled': {
            color: isDark ? theme.palette.grey[600] : theme.palette.grey[500],
            backgroundColor: isDark ? theme.palette.grey[800] : theme.palette.grey[100],
            borderColor: isDark ? theme.palette.grey[700] : theme.palette.grey[300],
          },
        },
        contained: {
          boxShadow: 'none',
          '&:hover': {
            boxShadow: 'none',
          },
          '&.Mui-disabled': {
            background: isDark ? theme.palette.grey[800] : theme.palette.grey[100],
            color: isDark ? theme.palette.grey[600] : theme.palette.grey[500],
          },
        },
        outlined: {
          borderWidth: '1.5px',
          '&:hover': {
            borderWidth: '1.5px',
          },
          '&.Mui-disabled': {
            borderColor: isDark ? theme.palette.grey[700] : theme.palette.grey[300],
            color: isDark ? theme.palette.grey[600] : theme.palette.grey[500],
          },
        },
      },
    },
    MuiPaper: {
      defaultProps: {
        elevation: 0,
      },
      styleOverrides: {
        root: {
          backgroundImage: 'none',
          border: `1px solid ${isDark ? alpha('#fff', 0.1) : theme.palette.grey[200]}`,
          backgroundColor: theme.palette.background.paper,
          boxShadow: 'none',
        },
        rounded: {
          borderRadius: `${borderRadius.lg}px`,
        },
      },
    },
    MuiAlert: {
      styleOverrides: {
        root: {
          alignItems: 'center',
        },
        outlined: {
          border: '1px dashed',
        },
      },
    },
    MuiListItemButton: {
      styleOverrides: {
        root: {
          color: theme.palette.text.primary,
          paddingTop: '10px',
          paddingBottom: '10px',
          '&.Mui-selected': {
            color: menuSelected,
            backgroundColor: menuSelectedBack,
            '&:hover': {
              backgroundColor: menuSelectedBack,
            },
            '& .MuiListItemIcon-root': {
              color: menuSelected,
            },
          },
          '&:hover': {
            backgroundColor: menuSelectedBack,
            color: menuSelected,
            '& .MuiListItemIcon-root': {
              color: menuSelected,
            },
          },
        },
      },
    },
    MuiListItemIcon: {
      styleOverrides: {
        root: {
          color: theme.palette.text.primary,
          minWidth: '36px',
        },
      },
    },
    MuiListItemText: {
      styleOverrides: {
        primary: {
          color: theme.palette.text.primary,
        },
      },
    },
    MuiInputBase: {
      styleOverrides: {
        input: {
          color: theme.palette.text.primary,
          '&::placeholder': {
            // Softer than full-strength secondary — avoids placeholders reading like real values.
            color: alpha(theme.palette.text.secondary, isDark ? 0.72 : 0.58),
            // Match the input's font-size (e.g. small fields use 0.8rem); a fixed
            // 0.875rem here made placeholders look vertically mis-centered.
            fontSize: 'inherit',
            lineHeight: 'inherit',
          },
        },
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          background: isDark ? alpha('#fff', 0.05) : '#ffffff',
          borderRadius: `${borderRadius.md}px`,
          transition: transitions.all,
          '& .MuiOutlinedInput-notchedOutline': {
            borderColor: isDark ? alpha('#fff', 0.2) : theme.palette.grey[300],
            borderWidth: '1.5px',
          },
          '&:hover .MuiOutlinedInput-notchedOutline': {
            borderColor: theme.palette.primary.main,
          },
          '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
            borderColor: theme.palette.primary.main,
            boxShadow: `0 0 0 2px ${alpha(theme.palette.primary.main, 0.1)}`,
          },
          '&.MuiInputBase-multiline': {
            padding: 1,
          },
        },
        input: {
          fontWeight: 400,
          padding: '12px 14px',
          fontSize: '0.875rem',
          '&::placeholder': {
            transform: 'none',
            lineHeight: 'inherit',
          },
          '&.MuiInputBase-inputSizeSmall': {
            padding: '10px 12px',
            fontSize: '0.8rem',
            '&.MuiInputBase-inputAdornedStart': {
              paddingLeft: 0,
            },
          },
        },
        inputAdornedStart: {
          paddingLeft: 4,
        },
        notchedOutline: {
          borderRadius: `${borderRadius.md}px`,
        },
      },
    },
    MuiDivider: {
      styleOverrides: {
        root: {
          borderColor: theme.palette.divider,
          opacity: isDark ? 0.4 : 1,
        },
      },
    },
    MuiSelect: {
      styleOverrides: {
        select: {
          '&:focus': {
            backgroundColor: 'transparent',
          },
        },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: {
          '&.MuiChip-deletable .MuiChip-deleteIcon': {
            color: 'inherit',
          },
        },
      },
    },
    MuiTooltip: {
      styleOverrides: {
        tooltip: {
          color: theme.palette.background.paper,
          background: theme.palette.text.primary,
        },
      },
    },
    MuiFormHelperText: {
      styleOverrides: {
        root: {
          color: isDark ? theme.palette.grey[400] : theme.palette.grey[700],
          fontSize: '0.75rem',
          marginTop: '2px',
          marginLeft: 0,
          marginRight: 0,
          lineHeight: 1.4,
        },
      },
    },
    MuiInputLabel: {
      styleOverrides: {
        root: {
          color: isDark ? theme.palette.grey[300] : theme.palette.grey[700],
          fontSize: '0.875rem',
          // Default (medium) outlined input
          transform: 'translate(14px, 12px) scale(1)',
          '&.MuiInputLabel-shrink': {
            transform: 'translate(14px, -9px) scale(0.75)',
          },
          // Align with MUI small OutlinedInput so labels/placeholders match field height
          '&.MuiInputLabel-sizeSmall': {
            transform: 'translate(14px, 9px) scale(1)',
            '&.MuiInputLabel-shrink': {
              transform: 'translate(14px, -9px) scale(0.75)',
            },
          },
          '&.Mui-focused': {
            color: theme.palette.primary.main,
          },
        },
      },
    },
  };
};

export default ComponentStyleOverrides;
