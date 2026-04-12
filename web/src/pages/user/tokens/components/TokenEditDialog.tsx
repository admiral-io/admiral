import { useState } from 'react';
import type { JSX } from 'react';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@mui/material';

import type { AccessToken } from '@/types/token';

import ScopeSelector from '@/pages/user/tokens/components/ScopeSelector';
import { validateTokenName } from '@/pages/user/tokens/utils';

export interface TokenEditDialogProps {
  open: boolean;
  token: AccessToken | null;
  loading: boolean;
  error?: string;
  onClose: () => void;
  onSubmit: (values: { tokenId: string; name: string; scopes: string[] }) => void;
}

export default function TokenEditDialog({
  open,
  token,
  loading,
  error,
  onClose,
  onSubmit,
}: TokenEditDialogProps): JSX.Element {
  const [name, setName] = useState('');
  const [scopes, setScopes] = useState<string[]>([]);

  const handleEnter = () => {
    if (token) {
      setName(token.name);
      setScopes([...token.scopes]);
    }
  };

  const handleExited = () => {
    setName('');
    setScopes([]);
  };

  const nameError = validateTokenName(name.trim());
  const canSubmit = !loading && !!name.trim() && !nameError && scopes.length > 0;

  const handleSubmit = () => {
    if (!token) return;
    onSubmit({ tokenId: token.id, name: name.trim(), scopes });
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && canSubmit) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="sm"
      slotProps={{ transition: { onEnter: handleEnter, onExited: handleExited } }}
      onKeyDown={handleKeyDown}
    >
      <DialogTitle>Edit Token</DialogTitle>
      <DialogContent>
        <Box sx={{ mt: 1, display: 'flex', flexDirection: 'column', gap: 2.5 }}>
          {error && <Alert severity="error">{error}</Alert>}

          <TextField
            label="Token name"
            size="small"
            fullWidth
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={name.length > 0 && !!nameError}
            helperText={name.length > 0 && nameError ? nameError : 'Lowercase letters, numbers, and hyphens (e.g. ci-deploy-token)'}
          />

          <Box>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Scopes
            </Typography>
            <ScopeSelector selected={scopes} onChange={setScopes} />
          </Box>
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={!canSubmit}
        >
          {loading ? 'Saving\u2026' : 'Save'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
