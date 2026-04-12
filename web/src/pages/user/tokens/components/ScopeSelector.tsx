import type { JSX } from 'react';
import { Box, Checkbox, Typography } from '@mui/material';

import {
  SCOPE_GROUPS,
  allScopeValues,
  groupScopeValues,
  isGroupFullySelected,
  isGroupPartiallySelected,
} from '@/pages/user/tokens/scopes';

interface ScopeSelectorProps {
  selected: string[];
  onChange: (scopes: string[]) => void;
}

export default function ScopeSelector({ selected, onChange }: ScopeSelectorProps): JSX.Element {
  const every = allScopeValues();
  const allSelected = every.length > 0 && every.every((s) => selected.includes(s));
  const someSelected = !allSelected && selected.length > 0;

  const toggleAll = () => {
    onChange(allSelected ? [] : [...every]);
  };

  const toggleScope = (scope: string) => {
    onChange(
      selected.includes(scope)
        ? selected.filter((s) => s !== scope)
        : [...selected, scope],
    );
  };

  const toggleGroup = (groupId: string) => {
    const group = SCOPE_GROUPS.find((g) => g.id === groupId);
    if (!group) return;

    const values = groupScopeValues(group);
    const groupAllSelected = isGroupFullySelected(group, selected);

    onChange(
      groupAllSelected
        ? selected.filter((s) => !values.includes(s))
        : [...new Set([...selected, ...values])],
    );
  };

  return (
    <Box
      sx={{
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        overflow: 'hidden',
      }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          px: 1.5,
          py: 0.75,
          cursor: 'pointer',
          bgcolor: 'action.hover',
          '&:hover': { bgcolor: 'action.selected' },
        }}
        onClick={toggleAll}
      >
        <Checkbox
          size="small"
          checked={allSelected}
          indeterminate={someSelected}
          tabIndex={-1}
          sx={{ p: 0.5 }}
        />
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          Select all
        </Typography>
      </Box>

      {SCOPE_GROUPS.map((group) => (
        <Box
          key={group.id}
          sx={{
            borderTop: 1,
            borderColor: 'divider',
          }}
        >
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1,
              px: 1.5,
              py: 0.75,
              cursor: 'pointer',
              '&:hover': { bgcolor: 'action.hover' },
            }}
            onClick={() => toggleGroup(group.id)}
          >
            <Checkbox
              size="small"
              checked={isGroupFullySelected(group, selected)}
              indeterminate={isGroupPartiallySelected(group, selected)}
              tabIndex={-1}
              sx={{ p: 0.5 }}
            />
            <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>
              {group.label}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {group.description}
            </Typography>
          </Box>

          {group.scopes.map((scope) => (
            <Box
              key={scope.value}
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                pl: 5,
                pr: 1.5,
                py: 0.5,
                cursor: 'pointer',
                '&:hover': { bgcolor: 'action.hover' },
              }}
              onClick={() => toggleScope(scope.value)}
            >
              <Checkbox
                size="small"
                checked={selected.includes(scope.value)}
                tabIndex={-1}
                sx={{ p: 0.5 }}
              />
              <Typography variant="body2" sx={{ fontFamily: 'monospace', minWidth: 140 }}>
                {scope.value}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {scope.label}
              </Typography>
            </Box>
          ))}
        </Box>
      ))}
    </Box>
  );
}
