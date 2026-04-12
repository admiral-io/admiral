import { useCallback, useState } from 'react';
import type { JSX } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
  useTheme,
} from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { useNavigate } from 'react-router-dom';

import { services } from '@/services';
import { openSnackbar } from '@/store/slices/snackbar';
import { useDispatch } from '@/store';
import type { CreateTokenResponse } from '@/types/token';

import ScopeSelector from '@/pages/user/tokens/components/ScopeSelector';
import { validateTokenName } from '@/pages/user/tokens/utils';

const EXPIRATION_OPTIONS = [
  { value: '7', label: '7 days' },
  { value: '30', label: '30 days' },
  { value: '60', label: '60 days' },
  { value: '90', label: '90 days' },
  { value: '365', label: '1 year' },
  { value: 'none', label: 'No expiration' },
] as const;

function computeExpiresAt(daysValue: string): string | undefined {
  if (daysValue === 'none') return undefined;
  const date = new Date();
  date.setDate(date.getDate() + parseInt(daysValue, 10));
  return date.toISOString();
}

function formatExpirationDate(daysValue: string): string {
  if (daysValue === 'none') return '';
  const date = new Date();
  date.setDate(date.getDate() + parseInt(daysValue, 10));
  return new Intl.DateTimeFormat('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  }).format(date);
}

export default function TokenCreatePage(): JSX.Element {
  const theme = useTheme();
  const navigate = useNavigate();
  const dispatch = useDispatch();

  const [name, setName] = useState('');
  const [scopes, setScopes] = useState<string[]>([]);
  const [expiration, setExpiration] = useState('30');
  const [loading, setLoading] = useState(false);

  const [createdToken, setCreatedToken] = useState<CreateTokenResponse>();
  const [copied, setCopied] = useState(false);

  const nameError = validateTokenName(name.trim());
  const canSubmit = !loading && !!name.trim() && !nameError && scopes.length > 0;

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && canSubmit) {
      e.preventDefault();
      void handleSubmit();
    } else if (e.key === 'Escape') {
      navigate('/user/tokens');
    }
  };

  const handleSubmit = useCallback(async () => {
    setLoading(true);
    try {
      const result = await services.token.create({
        name: name.trim(),
        scopes,
        expiresAt: computeExpiresAt(expiration),
      });
      setCreatedToken(result);
    } catch (err) {
      dispatch(
        openSnackbar({
          variant: 'alert',
          message: err instanceof Error ? err.message : String(err),
          alert: { color: 'error', variant: 'filled' },
        }),
      );
    } finally {
      setLoading(false);
    }
  }, [dispatch, name, scopes, expiration]);

  const handleCopy = async () => {
    if (!createdToken) return;
    await navigator.clipboard.writeText(createdToken.plain_text_token);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (createdToken) {
    return (
      <Stack spacing={3} sx={{ maxWidth: 720 }}>
        <Typography variant="h5" component="h1">
          Personal Access Token Created
        </Typography>

        <Alert severity="warning" variant="outlined">
          Copy your new personal access token now. You won&apos;t be able to see it again.
        </Alert>

        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1.5,
            p: 2,
            borderRadius: 1,
            border: `1px solid ${theme.palette.success.main}`,
            bgcolor: theme.palette.mode === 'dark'
              ? 'rgba(46, 125, 50, 0.08)'
              : 'rgba(46, 125, 50, 0.04)',
          }}
        >
          <Typography
            variant="body2"
            sx={{ flex: 1, fontFamily: 'monospace', wordBreak: 'break-all' }}
          >
            {createdToken.plain_text_token}
          </Typography>
          <Button
            size="small"
            variant="outlined"
            startIcon={<ContentCopyIcon />}
            onClick={() => void handleCopy()}
            color={copied ? 'success' : 'primary'}
            sx={{ flexShrink: 0 }}
          >
            {copied ? 'Copied' : 'Copy'}
          </Button>
        </Box>

        <Box>
          <Typography variant="subtitle2" sx={{ mb: 1 }}>
            Scopes
          </Typography>
          <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
            {createdToken.access_token.scopes.map((scope) => (
              <Chip key={scope} label={scope} size="small" variant="outlined" />
            ))}
          </Stack>
        </Box>

        <Box>
          <Button
            variant="contained"
            startIcon={<ArrowBackIcon />}
            onClick={() => navigate('/user/tokens')}
          >
            Back to tokens
          </Button>
        </Box>
      </Stack>
    );
  }

  const expirationHint = expiration !== 'none'
    ? `The token will expire on ${formatExpirationDate(expiration)}.`
    : 'The token will never expire.';

  return (
    <Stack spacing={3} sx={{ maxWidth: 720 }} onKeyDown={handleKeyDown}>
      <Box>
        <Typography variant="h5" component="h1">
          New personal access token
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          Personal access tokens function like API keys. They can be used to authenticate with the
          Admiral API.
        </Typography>
      </Box>

      <Box>
        <Typography variant="subtitle2" sx={{ mb: 1 }}>
          Note
        </Typography>
        <TextField
          size="small"
          fullWidth
          placeholder="e.g. ci-deploy-token"
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={name.length > 0 && !!nameError}
          helperText={name.length > 0 && nameError ? nameError : 'Lowercase letters, numbers, and hyphens (e.g. ci-deploy-token)'}
        />
      </Box>

      <Box>
        <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
          Expiration
        </Typography>
        <FormControl size="small" sx={{ minWidth: 240 }}>
          <InputLabel>Expiration</InputLabel>
          <Select
            value={expiration}
            label="Expiration"
            onChange={(e) => setExpiration(e.target.value)}
          >
            {EXPIRATION_OPTIONS.map(({ value, label }) => (
              <MenuItem key={value} value={value}>{label}</MenuItem>
            ))}
          </Select>
        </FormControl>
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
          {expirationHint}
        </Typography>
      </Box>

      <Box>
        <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 0.5 }}>
          Select scopes
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          Scopes define the access for personal tokens.
        </Typography>
        <ScopeSelector selected={scopes} onChange={setScopes} />
      </Box>

      <Stack direction="row" spacing={1.5}>
        <Button
          variant="contained"
          onClick={() => void handleSubmit()}
          disabled={!canSubmit}
        >
          {loading ? 'Generating token\u2026' : 'Generate token'}
        </Button>
        <Button
          variant="outlined"
          onClick={() => navigate('/user/tokens')}
          disabled={loading}
        >
          Cancel
        </Button>
      </Stack>
    </Stack>
  );
}
