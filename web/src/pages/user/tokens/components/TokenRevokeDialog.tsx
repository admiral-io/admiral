import type { JSX } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material';

import type { AccessToken } from '@/types/token';

export interface TokenRevokeDialogProps {
  open: boolean;
  token: AccessToken | null;
  loading: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

export default function TokenRevokeDialog({
  open,
  token,
  loading,
  onClose,
  onConfirm,
}: TokenRevokeDialogProps): JSX.Element {
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle>Revoke token?</DialogTitle>
      <DialogContent>
        <DialogContentText>
          This will permanently revoke <strong>{token?.name ?? 'this token'}</strong>. Any
          applications or scripts using this token will lose access immediately. This action
          cannot be undone.
        </DialogContentText>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button color="error" variant="contained" onClick={onConfirm} disabled={loading}>
          {loading ? 'Revoking\u2026' : 'Revoke token'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
