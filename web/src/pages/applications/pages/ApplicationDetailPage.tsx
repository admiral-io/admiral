import { useCallback, useEffect, useState } from 'react';
import type { JSX } from 'react';
import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  Chip,
  Divider,
  Link as MuiLink,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';

import type { Application } from '@/types/application';
import { openSnackbar } from '@/store/slices/snackbar';
import { useDispatch } from '@/store';
import { services } from '@/services';

import ApplicationDeleteDialog from '@/pages/applications/components/ApplicationDeleteDialog';
import ApplicationFormDialog from '@/pages/applications/components/ApplicationFormDialog';
import { formatShortDate } from '@/pages/applications/utils';

/**
 * Dedicated application hub: environments, workloads, variables, etc. will grow here.
 */
export default function ApplicationDetailPage(): JSX.Element {
  const { applicationId } = useParams<{ applicationId: string }>();
  const navigate = useNavigate();
  const dispatch = useDispatch();

  const [application, setApplication] = useState<Application>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const [formOpen, setFormOpen] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [formError, setFormError] = useState<string>();

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const [envCount, setEnvCount] = useState<number | undefined>();
  const [envNamesLoaded, setEnvNamesLoaded] = useState(false);

  const load = useCallback(async () => {
    if (!applicationId) return;
    setLoading(true);
    setError(undefined);
    try {
      const app = await services.application.get(applicationId);
      setApplication(app);

      try {
        const envs = await services.environment.listAll();
        const byApp = services.environment.countByApplicationId(envs);
        setEnvCount(byApp.get(applicationId) ?? 0);
        setEnvNamesLoaded(true);
      } catch {
        setEnvCount(undefined);
        setEnvNamesLoaded(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [applicationId]);

  useEffect(() => {
    void load();
  }, [load]);

  const canDelete = envNamesLoaded && (envCount ?? 0) === 0;

  const handleEditSubmit = useCallback(
    async (values: { name: string; description: string; labels: Record<string, string> | undefined }) => {
      if (!application) return;
      setFormLoading(true);
      setFormError(undefined);
      try {
        const updated = await services.application.update({
          id: application.id,
          name: application.name,
          description: values.description.trim(),
          labels: values.labels,
        });
        setApplication(updated);
        setFormOpen(false);
        dispatch(
          openSnackbar({
            variant: 'alert',
            message: 'Application updated',
            alert: { color: 'success', variant: 'filled' },
          }),
        );
      } catch (err) {
        setFormError(err instanceof Error ? err.message : String(err));
      } finally {
        setFormLoading(false);
      }
    },
    [application, dispatch],
  );

  const handleDeleteConfirm = useCallback(async () => {
    if (!application) return;
    setDeleteLoading(true);
    try {
      await services.application.remove(application.id);
      dispatch(
        openSnackbar({
          variant: 'alert',
          message: 'Application deleted',
          alert: { color: 'success', variant: 'filled' },
        }),
      );
      navigate('/applications');
    } catch (err) {
      dispatch(
        openSnackbar({
          variant: 'alert',
          message: err instanceof Error ? err.message : String(err),
          alert: { color: 'error', variant: 'filled' },
        }),
      );
    } finally {
      setDeleteLoading(false);
      setDeleteOpen(false);
    }
  }, [application, dispatch, navigate]);

  if (!applicationId) {
    return <Alert severity="error">Missing application id.</Alert>;
  }

  if (loading && !application) {
    return (
      <Typography variant="body2" color="text.secondary">
        Loading…
      </Typography>
    );
  }

  if (error || !application) {
    return (
      <Stack spacing={2}>
        <Button component={RouterLink} to="/applications" startIcon={<ArrowBackIcon />} variant="outlined" size="small">
          Back to applications
        </Button>
        <Alert severity="error">{error ?? 'Application not found.'}</Alert>
      </Stack>
    );
  }

  return (
    <Stack spacing={3}>
      <Breadcrumbs aria-label="breadcrumb">
        <MuiLink component={RouterLink} to="/applications" underline="hover" color="inherit">
          Applications
        </MuiLink>
        <Typography color="text.primary">{application.name}</Typography>
      </Breadcrumbs>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'flex-start' }} justifyContent="space-between">
        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {application.name}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 720 }}>
            {application.description || 'No description'}
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1, fontFamily: 'monospace' }}>
            {application.id}
          </Typography>
          <Stack direction="row" flexWrap="wrap" gap={0.5} sx={{ mt: 1.5 }}>
            {application.labels && Object.keys(application.labels).length > 0 ? (
              Object.entries(application.labels).map(([k, v]) => (
                <Chip key={k} size="small" label={`${k}=${v}`} variant="outlined" />
              ))
            ) : (
              <Typography variant="caption" color="text.secondary">
                No labels
              </Typography>
            )}
          </Stack>
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
            Updated {formatShortDate(application.updated_at)}
          </Typography>
        </Box>
        <Stack direction="row" spacing={1} flexShrink={0}>
          <Button variant="outlined" startIcon={<EditOutlinedIcon />} onClick={() => setFormOpen(true)}>
            Edit
          </Button>
          <Button
            color="error"
            variant="outlined"
            startIcon={<DeleteOutlineIcon />}
            disabled={!canDelete}
            onClick={() => setDeleteOpen(true)}
          >
            Delete
          </Button>
        </Stack>
      </Stack>

      {!envNamesLoaded && (
        <Alert severity="info">
          Environment count could not be loaded. Delete may be unavailable until you refresh.
        </Alert>
      )}
      {envNamesLoaded && !canDelete && (envCount ?? 0) > 0 && (
        <Alert severity="warning">
          This application has {envCount} environment{envCount === 1 ? '' : 's'}. Remove them before you can delete
          the application.
        </Alert>
      )}

      <Divider />

      <Typography variant="h6" component="h2">
        Environments
      </Typography>
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Create and manage environments here (coming next). This section will list clusters, namespaces, and
          deployment targets for this application.
        </Typography>
      </Paper>

      <Typography variant="h6" component="h2">
        Workloads
      </Typography>
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Typography variant="body2" color="text.secondary">
          View workloads and deployment status (placeholder).
        </Typography>
      </Paper>

      <Typography variant="h6" component="h2">
        Variables
      </Typography>
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Configure application and environment variables (placeholder).
        </Typography>
      </Paper>

      <ApplicationFormDialog
        open={formOpen}
        mode="edit"
        initial={application}
        loading={formLoading}
        error={formError}
        onClose={() => {
          if (!formLoading) setFormOpen(false);
        }}
        onSubmit={handleEditSubmit}
      />

      <ApplicationDeleteDialog
        open={deleteOpen}
        application={application}
        loading={deleteLoading}
        canDelete={canDelete}
        environmentCount={envNamesLoaded ? (envCount ?? 0) : undefined}
        onClose={() => !deleteLoading && setDeleteOpen(false)}
        onConfirm={handleDeleteConfirm}
      />
    </Stack>
  );
}
