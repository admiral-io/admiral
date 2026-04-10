import type { JSX } from 'react';
import {
  Box,
  Button,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import RefreshIcon from '@mui/icons-material/Refresh';
import ViewListIcon from '@mui/icons-material/ViewList';
import ViewModuleIcon from '@mui/icons-material/ViewModule';

import type { ApplicationsSortField, SortDir } from '@/pages/applications/hooks/useApplicationsResource';

export type ApplicationsLayoutVariant = 'table' | 'cards';

interface ApplicationToolbarProps {
  loading: boolean;
  onRefresh: () => void;
  onCreate: () => void;
  variant: ApplicationsLayoutVariant;
  onVariantChange: (v: ApplicationsLayoutVariant) => void;
  sortField: ApplicationsSortField;
  sortDir: SortDir;
  onSortFieldChange: (field: ApplicationsSortField) => void;
  onSortDirChange: (dir: SortDir) => void;
  showSortControls: boolean;
}

export default function ApplicationToolbar({
  loading,
  onRefresh,
  onCreate,
  variant,
  onVariantChange,
  sortField,
  sortDir,
  onSortFieldChange,
  onSortDirChange,
  showSortControls,
}: ApplicationToolbarProps): JSX.Element {
  return (
    <Box
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        gap: 2,
        rowGap: 1.5,
      }}
    >
      <Typography variant="h4" component="h1" sx={{ flexShrink: 0 }}>
        Applications
      </Typography>

      {showSortControls && (
        <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1.5 }}>
          <FormControl size="small" sx={{ minWidth: 150, flex: { xs: '1 1 140px', sm: '0 0 auto' } }}>
            <InputLabel id="applications-sort-field">Sort by</InputLabel>
            <Select
              labelId="applications-sort-field"
              label="Sort by"
              value={sortField}
              onChange={(e) => onSortFieldChange(e.target.value as ApplicationsSortField)}
            >
              <MenuItem value="name">Name</MenuItem>
              <MenuItem value="updated_at">Updated</MenuItem>
              <MenuItem value="created_at">Created</MenuItem>
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 112, flex: { xs: '1 1 100px', sm: '0 0 auto' } }}>
            <InputLabel id="applications-sort-dir">Order</InputLabel>
            <Select
              labelId="applications-sort-dir"
              label="Order"
              value={sortDir}
              onChange={(e) => onSortDirChange(e.target.value as SortDir)}
            >
              <MenuItem value="asc">Ascending</MenuItem>
              <MenuItem value="desc">Descending</MenuItem>
            </Select>
          </FormControl>
        </Box>
      )}

      <Box sx={{ flex: '1 1 0', minWidth: 0 }} aria-hidden />

      <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1 }}>
        <ToggleButtonGroup
          value={variant}
          exclusive
          size="small"
          onChange={(_, v: ApplicationsLayoutVariant | null) => v && onVariantChange(v)}
          aria-label="layout"
        >
          <ToggleButton value="table" aria-label="table layout">
            <Tooltip title="Table layout">
              <ViewListIcon fontSize="small" />
            </Tooltip>
          </ToggleButton>
          <ToggleButton value="cards" aria-label="card layout">
            <Tooltip title="Card layout">
              <ViewModuleIcon fontSize="small" />
            </Tooltip>
          </ToggleButton>
        </ToggleButtonGroup>
        <Button
          variant="outlined"
          size="small"
          startIcon={<RefreshIcon />}
          onClick={() => void onRefresh()}
          disabled={loading}
        >
          Refresh
        </Button>
        <Button variant="contained" size="small" startIcon={<AddIcon />} onClick={onCreate}>
          Create
        </Button>
      </Box>
    </Box>
  );
}
