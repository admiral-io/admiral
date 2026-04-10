import { useState } from 'react';
import type { JSX } from 'react';
import {
  Box,
  Chip,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TableSortLabel,
  Typography,
} from '@mui/material';

import type { Application } from '@/types/application';

import type { ApplicationsSortField, SortDir } from '@/pages/applications/hooks/useApplicationsResource';
import { formatShortDate } from '@/pages/applications/utils';

const MAX_ENV_CHIPS = 4;

function EnvironmentCell({ names }: { names: string[] | undefined }) {
  if (names === undefined) {
    return (
      <Typography variant="caption" color="text.secondary">
        —
      </Typography>
    );
  }
  if (names.length === 0) {
    return (
      <Typography variant="caption" color="text.secondary">
        None
      </Typography>
    );
  }
  const shown = names.slice(0, MAX_ENV_CHIPS);
  const rest = names.length - shown.length;
  return (
    <Stack direction="row" flexWrap="wrap" gap={0.5} useFlexGap sx={{ maxWidth: 360 }}>
      {shown.map((n, i) => (
        <Chip key={`${n}-${i}`} size="small" label={n} variant="outlined" />
      ))}
      {rest > 0 && <Chip size="small" label={`+${rest} more`} variant="outlined" />}
    </Stack>
  );
}

interface ApplicationsTableVariantProps {
  rows: Application[];
  loading: boolean;
  sortField: ApplicationsSortField;
  sortDir: SortDir;
  requestSort: (field: ApplicationsSortField) => void;
  environmentNames: (applicationId: string) => string[] | undefined;
  onRowNavigate: (app: Application) => void;
}

export default function ApplicationsTableVariant({
  rows,
  loading,
  sortField,
  sortDir,
  requestSort,
  environmentNames,
  onRowNavigate,
}: ApplicationsTableVariantProps): JSX.Element {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);

  const paginated = rows.slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage);

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <TableContainer>
        <Table size="small" stickyHeader aria-label="applications table">
          <TableHead>
            <TableRow>
              <TableCell sortDirection={sortField === 'name' ? sortDir : false}>
                <TableSortLabel
                  active={sortField === 'name'}
                  direction={sortField === 'name' ? sortDir : 'asc'}
                  onClick={() => requestSort('name')}
                >
                  Name
                </TableSortLabel>
              </TableCell>
              <TableCell>Description</TableCell>
              <TableCell>Environments</TableCell>
              <TableCell sortDirection={sortField === 'updated_at' ? sortDir : false}>
                <TableSortLabel
                  active={sortField === 'updated_at'}
                  direction={sortField === 'updated_at' ? sortDir : 'asc'}
                  onClick={() => requestSort('updated_at')}
                >
                  Updated
                </TableSortLabel>
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {loading && rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4}>
                  <Typography variant="body2" color="text.secondary" sx={{ py: 3 }}>
                    Loading…
                  </Typography>
                </TableCell>
              </TableRow>
            ) : paginated.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4}>
                  <Typography variant="body2" color="text.secondary" sx={{ py: 3 }}>
                    No applications match your filters.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              paginated.map((app) => (
                <TableRow
                  key={app.id}
                  hover
                  onClick={() => onRowNavigate(app)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      onRowNavigate(app);
                    }
                  }}
                  tabIndex={0}
                  role="link"
                  aria-label={`Open ${app.name}`}
                  sx={{ cursor: 'pointer' }}
                >
                  <TableCell sx={{ fontWeight: 600 }}>{app.name}</TableCell>
                  <TableCell sx={{ maxWidth: 320 }}>
                    <Typography variant="body2" noWrap title={app.description}>
                      {app.description || '—'}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <EnvironmentCell names={environmentNames(app.id)} />
                  </TableCell>
                  <TableCell>{formatShortDate(app.updated_at)}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <Box sx={{ borderTop: 1, borderColor: 'divider' }}>
        <TablePagination
          component="div"
          rowsPerPageOptions={[5, 10, 25, 50]}
          count={rows.length}
          rowsPerPage={rowsPerPage}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(parseInt(e.target.value, 10));
            setPage(0);
          }}
        />
      </Box>
    </Paper>
  );
}
