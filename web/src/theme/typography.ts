import type { Theme, TypographyVariantsOptions } from '@mui/material/styles';

const Typography = (theme: Theme): TypographyVariantsOptions => {
  const headingColor =
    theme.palette.mode === 'dark' ? theme.palette.text.primary : theme.palette.grey[900];

  return {
    fontFamily: "'Geist Variable', 'Helvetica', 'Arial', sans-serif",
    h6: {
      fontWeight: 600,
      color: headingColor,
      fontSize: '0.85rem',
      lineHeight: 1.4,
      letterSpacing: '0.01em',
    },
    h5: {
      fontSize: '0.95rem',
      color: headingColor,
      fontWeight: 600,
      lineHeight: 1.4,
      letterSpacing: '0.01em',
    },
    h4: {
      fontSize: '1.1rem',
      color: headingColor,
      fontWeight: 600,
      lineHeight: 1.3,
      letterSpacing: '0.005em',
    },
    h3: {
      fontSize: '1.3rem',
      color: headingColor,
      fontWeight: 700,
      lineHeight: 1.3,
      letterSpacing: '0.005em',
    },
    h2: {
      fontSize: '1.6rem',
      color: headingColor,
      fontWeight: 700,
      lineHeight: 1.2,
      letterSpacing: '-0.005em',
    },
    h1: {
      fontSize: '2rem',
      color: headingColor,
      fontWeight: 800,
      lineHeight: 1.15,
      letterSpacing: '-0.01em',
    },
    subtitle1: {
      fontSize: '0.875rem',
      fontWeight: 500,
    },
    subtitle2: {
      fontSize: '0.75rem',
      fontWeight: 400,
      color: theme.palette.text.secondary,
    },
    caption: {
      fontSize: '0.75rem',
      color: theme.palette.text.secondary,
      fontWeight: 400,
    },
    body1: {
      fontSize: '0.875rem',
      fontWeight: 400,
      lineHeight: 1.5,
      letterSpacing: '0.005em',
      color: theme.palette.text.primary,
    },
    body2: {
      fontSize: '0.8rem',
      letterSpacing: '0.005em',
      fontWeight: 400,
      lineHeight: 1.4,
      color: theme.palette.text.secondary,
    },
    button: {
      textTransform: 'none',
      fontWeight: 600,
      letterSpacing: '0.02em',
    },
    overline: {
      fontSize: '0.625rem',
      fontWeight: 600,
      letterSpacing: '0.08em',
      textTransform: 'uppercase',
      color: theme.palette.text.secondary,
    },
  };
};

export default Typography;
