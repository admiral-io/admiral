import type { JSX } from 'react';
import { Card, CardContent, Chip, Grid, Stack, Typography } from '@mui/material';

import type { Application } from '@/types/application';

import { formatShortDate } from '@/pages/applications/utils';

const MAX_ENV_CHIPS = 5;

interface ApplicationsCardGridVariantProps {
  rows: Application[];
  loading: boolean;
  environmentNames: (applicationId: string) => string[] | undefined;
  onCardNavigate: (app: Application) => void;
}

export default function ApplicationsCardGridVariant({
  rows,
  loading,
  environmentNames,
  onCardNavigate,
}: ApplicationsCardGridVariantProps): JSX.Element {
  if (loading && rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{ py: 4 }}>
        Loading…
      </Typography>
    );
  }

  if (rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{ py: 4 }}>
        No applications match your filters.
      </Typography>
    );
  }

  return (
    <Grid container spacing={2} columns={12}>
      {rows.map((app) => {
        const envs = environmentNames(app.id);
        const shown = envs?.slice(0, MAX_ENV_CHIPS) ?? [];
        const rest = envs ? Math.max(0, envs.length - shown.length) : 0;
        return (
          <Grid key={app.id} size={{ xs: 12, sm: 6, lg: 4 }}>
            <Card
              variant="outlined"
              sx={{
                height: '100%',
                display: 'flex',
                flexDirection: 'column',
                cursor: 'pointer',
                transition: (theme) => theme.transitions.create(['box-shadow', 'border-color'], { duration: 200 }),
                '&:hover': {
                  borderColor: 'primary.main',
                  boxShadow: 1,
                },
              }}
              onClick={() => onCardNavigate(app)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  onCardNavigate(app);
                }
              }}
              tabIndex={0}
              role="link"
              aria-label={`Open ${app.name}`}
            >
              <CardContent sx={{ flex: 1 }}>
                <Typography variant="subtitle1" fontWeight={700} gutterBottom>
                  {app.name}
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5, minHeight: 40 }}>
                  {app.description || 'No description'}
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.75 }}>
                  Application labels
                </Typography>
                <Stack direction="row" flexWrap="wrap" gap={0.5} sx={{ mb: 1.5 }}>
                  {app.labels && Object.keys(app.labels).length > 0 ? (
                    Object.entries(app.labels).map(([k, v]) => (
                      <Chip key={k} size="small" label={`${k}=${v}`} variant="outlined" />
                    ))
                  ) : (
                    <Typography variant="caption" color="text.secondary">
                      None
                    </Typography>
                  )}
                </Stack>
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.75 }}>
                  Environments
                </Typography>
                <Stack direction="row" flexWrap="wrap" gap={0.5} sx={{ mb: 1 }}>
                  {envs === undefined ? (
                    <Typography variant="caption" color="text.secondary">
                      —
                    </Typography>
                  ) : envs.length === 0 ? (
                    <Typography variant="caption" color="text.secondary">
                      None yet
                    </Typography>
                  ) : (
                    <>
                      {shown.map((n, i) => (
                        <Chip key={`${n}-${i}`} size="small" label={n} variant="outlined" color="primary" />
                      ))}
                      {rest > 0 && <Chip size="small" label={`+${rest}`} variant="outlined" />}
                    </>
                  )}
                </Stack>
                <Typography variant="caption" color="text.secondary">
                  Updated {formatShortDate(app.updated_at)}
                </Typography>
              </CardContent>
            </Card>
          </Grid>
        );
      })}
    </Grid>
  );
}
