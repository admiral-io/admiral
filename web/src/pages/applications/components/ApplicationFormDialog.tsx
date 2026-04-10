import { useEffect, useState } from 'react';
import type { JSX } from 'react';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
} from '@mui/material';

import type { Application } from '@/types/application';

import { formatLabelsToString, parseLabelsFromString } from '@/pages/applications/utils';

export interface ApplicationFormDialogProps {
  open: boolean;
  mode: 'create' | 'edit';
  initial?: Application;
  loading?: boolean;
  error?: string;
  onClose: () => void;
  onSubmit: (values: {
    name: string;
    description: string;
    labels: Record<string, string> | undefined;
  }) => void | Promise<void>;
}

export default function ApplicationFormDialog({
  open,
  mode,
  initial,
  loading,
  error,
  onClose,
  onSubmit,
}: ApplicationFormDialogProps): JSX.Element {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [labels, setLabels] = useState('');

  useEffect(() => {
    if (!open) return;
    if (mode === 'edit' && initial) {
      setName(initial.name);
      setDescription(initial.description ?? '');
      setLabels(formatLabelsToString(initial.labels));
    } else {
      setName('');
      setDescription('');
      setLabels('');
    }
  }, [open, mode, initial]);

  const handleSubmit = () => {
    void onSubmit({
      name: name.trim(),
      description: description.trim(),
      labels: parseLabelsFromString(labels),
    });
  };

  const title = mode === 'create' ? 'Create application' : 'Edit application';

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField
            label="Name"
            size="small"
            fullWidth
            required
            placeholder="e.g. inventory-api"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={mode === 'edit'}
            helperText={mode === 'edit' ? 'Application name cannot be changed.' : undefined}
          />
          <TextField
            label="Description"
            size="small"
            fullWidth
            multiline
            minRows={2}
            placeholder="Optional description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <TextField
            label="Labels"
            size="small"
            fullWidth
            placeholder="e.g. team=logistics, tier=critical"
            value={labels}
            onChange={(e) => setLabels(e.target.value)}
            helperText="Comma-separated key=value pairs"
          />
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={loading || !name.trim()}
        >
          {loading ? 'Saving…' : mode === 'create' ? 'Create' : 'Save'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
