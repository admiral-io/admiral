import type { JSX } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material';

import type { Application } from '@/types/application';

export interface ApplicationDeleteDialogProps {
  open: boolean;
  application: Application | null;
  loading?: boolean;
  canDelete: boolean;
  environmentCount: number | undefined;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
}

export default function ApplicationDeleteDialog({
  open,
  application,
  loading,
  canDelete,
  environmentCount,
  onClose,
  onConfirm,
}: ApplicationDeleteDialogProps): JSX.Element {
  const name = application?.name ?? '';

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle>Delete application?</DialogTitle>
      <DialogContent>
        <DialogContentText component="div">
          {canDelete ? (
            <>
              This will permanently delete <strong>{name}</strong>. This action cannot be undone.
            </>
          ) : environmentCount === undefined ? (
            <>
              Environment counts are not available, so delete is disabled. Fix the environment list load (e.g.
              permissions) and refresh, then try again.
            </>
          ) : (
            <>
              <strong>{name}</strong> has {environmentCount} environment
              {environmentCount === 1 ? '' : 's'} assigned. Remove all environments first, then you can delete
              the application.
            </>
          )}
        </DialogContentText>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button color="error" variant="contained" onClick={() => void onConfirm()} disabled={loading || !canDelete}>
          {loading ? 'Deleting…' : 'Delete'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
