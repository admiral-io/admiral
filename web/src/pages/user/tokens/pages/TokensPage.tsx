import type { JSX } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';
import RefreshIcon from '@mui/icons-material/Refresh';
import { useNavigate } from 'react-router-dom';

import TokenEditDialog from '@/pages/user/tokens/components/TokenEditDialog';
import TokenRevokeDialog from '@/pages/user/tokens/components/TokenRevokeDialog';
import { formatTokenDate, resolveTokenStatus } from '@/pages/user/tokens/utils';
import { useTokensResource } from '@/pages/user/tokens/hooks/useTokensResource';

const statusColorMap = {
  active: 'success',
  expired: 'warning',
  revoked: 'error',
} as const;

export default function TokensPage(): JSX.Element {
  const navigate = useNavigate();
  const {
    tokens,
    loading,
    error,
    refresh,
    editOpen,
    editTarget,
    editLoading,
    editError,
    openEdit,
    closeEdit,
    submitEdit,
    revokeOpen,
    revokeTarget,
    revokeLoading,
    openRevoke,
    closeRevoke,
    confirmRevoke,
  } = useTokensResource();

  return (
    <Stack spacing={3}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Box>
          <Typography variant="h6" component="h2">
            Personal Access Tokens
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Tokens allow programmatic access to the Admiral API. Treat them like passwords.
          </Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          <Tooltip title="Refresh">
            <IconButton onClick={() => void refresh()} disabled={loading} size="small">
              <RefreshIcon />
            </IconButton>
          </Tooltip>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            size="small"
            onClick={() => navigate('/user/tokens/new')}
          >
            Generate new token
          </Button>
        </Stack>
      </Stack>

      {error && <Alert severity="error">{error}</Alert>}

      <TableContainer component={Paper} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Scopes</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Expires</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {!loading && tokens.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                  <Typography variant="body2" color="text.secondary">
                    No tokens yet. Create one to get started.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {loading && tokens.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                  <Typography variant="body2" color="text.secondary">
                    Loading&hellip;
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {tokens.map((token) => {
              const status = resolveTokenStatus(token);
              const isRevoked = status === 'revoked';
              return (
                <TableRow key={token.id} sx={{ opacity: isRevoked ? 0.5 : 1 }}>
                  <TableCell>
                    <Typography variant="body2" fontWeight={500}>
                      {token.name}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                      {token.scopes.map((scope) => (
                        <Chip key={scope} label={scope} size="small" variant="outlined" />
                      ))}
                    </Stack>
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={status}
                      size="small"
                      color={statusColorMap[status]}
                      variant="outlined"
                    />
                  </TableCell>
                  <TableCell>
                    <Typography variant="caption">
                      {formatTokenDate(token.created_at)}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="caption">
                      {formatTokenDate(token.expires_at)}
                    </Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                      <Tooltip title="Edit">
                        <span>
                          <IconButton
                            size="small"
                            onClick={() => openEdit(token)}
                            disabled={isRevoked}
                          >
                            <EditOutlinedIcon fontSize="small" />
                          </IconButton>
                        </span>
                      </Tooltip>
                      <Tooltip title="Revoke">
                        <span>
                          <IconButton
                            size="small"
                            color="error"
                            onClick={() => openRevoke(token)}
                            disabled={isRevoked}
                          >
                            <DeleteOutlineIcon fontSize="small" />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </Stack>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>

      <TokenEditDialog
        open={editOpen}
        token={editTarget}
        loading={editLoading}
        error={editError}
        onClose={closeEdit}
        onSubmit={(v) => void submitEdit(v)}
      />

      <TokenRevokeDialog
        open={revokeOpen}
        token={revokeTarget}
        loading={revokeLoading}
        onClose={closeRevoke}
        onConfirm={() => void confirmRevoke()}
      />
    </Stack>
  );
}