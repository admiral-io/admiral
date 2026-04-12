import { useCallback, useState } from 'react';
import type { JSX } from 'react';
import { Alert, Stack } from '@mui/material';
import { useNavigate } from 'react-router-dom';

import type { Application } from '@/types/application';
import { openSnackbar } from '@/store/slices/snackbar';
import { useDispatch } from '@/store';

import ApplicationFormDialog from '@/pages/applications/components/ApplicationFormDialog';
import ApplicationToolbar, {
  type ApplicationsLayoutVariant,
} from '@/pages/applications/components/ApplicationToolbar';
import ApplicationsCardGridVariant from '@/pages/applications/components/ApplicationsCardGridVariant';
import ApplicationsTableVariant from '@/pages/applications/components/ApplicationsTableVariant';
import { useApplicationsResource } from '@/pages/applications/hooks/useApplicationsResource';

/**
 * Applications list: table or cards, row/card opens `/applications/:id`.
 */
export default function ApplicationsPage(): JSX.Element {
  const dispatch = useDispatch();
  const navigate = useNavigate();
  const resource = useApplicationsResource();

  const [layoutVariant, setLayoutVariant] = useState<ApplicationsLayoutVariant>('table');
  const [createOpen, setCreateOpen] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [formError, setFormError] = useState<string>();

  const openCreate = useCallback(() => {
    setFormError(undefined);
    setCreateOpen(true);
  }, []);

  const goToApplication = useCallback(
    (app: Application) => {
      navigate(`/applications/${app.id}`);
    },
    [navigate],
  );

  const handleCreateSubmit = useCallback(
    async (values: { name: string; description: string; labels: Record<string, string> | undefined }) => {
      setFormLoading(true);
      setFormError(undefined);
      try {
        const created = await resource.create({
          name: values.name,
          description: values.description || undefined,
          labels: values.labels,
        });
        dispatch(
          openSnackbar({
            variant: 'alert',
            message: 'Application created',
            alert: { color: 'success', variant: 'filled' },
          }),
        );
        setCreateOpen(false);
        navigate(`/applications/${created.id}`);
      } catch (err) {
        setFormError(err instanceof Error ? err.message : String(err));
      } finally {
        setFormLoading(false);
      }
    },
    [dispatch, navigate, resource],
  );

  return (
    <Stack spacing={3}>
      <ApplicationToolbar
        loading={resource.loading}
        onRefresh={() => void resource.refresh()}
        onCreate={openCreate}
        variant={layoutVariant}
        onVariantChange={setLayoutVariant}
        sortField={resource.sortField}
        sortDir={resource.sortDir}
        onSortFieldChange={resource.setSortField}
        onSortDirChange={resource.setSortDir}
        showSortControls={layoutVariant === 'cards'}
      />

      {resource.error && <Alert severity="error">{resource.error}</Alert>}
      {resource.warning && <Alert severity="warning">{resource.warning}</Alert>}

      {layoutVariant === 'table' ? (
        <ApplicationsTableVariant
          rows={resource.rows}
          loading={resource.loading}
          sortField={resource.sortField}
          sortDir={resource.sortDir}
          requestSort={resource.requestSort}
          environmentNames={resource.environmentNames}
          onRowNavigate={goToApplication}
        />
      ) : (
        <ApplicationsCardGridVariant
          rows={resource.rows}
          loading={resource.loading}
          environmentNames={resource.environmentNames}
          onCardNavigate={goToApplication}
        />
      )}

      <ApplicationFormDialog
        open={createOpen}
        mode="create"
        loading={formLoading}
        error={formError}
        onClose={() => {
          if (!formLoading) setCreateOpen(false);
        }}
        onSubmit={handleCreateSubmit}
      />
    </Stack>
  );
}
