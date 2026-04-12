export interface ScopeDefinition {
  value: string;
  label: string;
}

export interface ScopeGroup {
  id: string;
  label: string;
  description: string;
  scopes: ScopeDefinition[];
}

export const SCOPE_GROUPS: ScopeGroup[] = [
  {
    id: 'app',
    label: 'app',
    description: 'Manage applications',
    scopes: [
      { value: 'app:read', label: 'View applications and their configuration' },
      { value: 'app:write', label: 'Create, update, and delete applications' },
    ],
  },
  {
    id: 'env',
    label: 'env',
    description: 'Manage deployment environments',
    scopes: [
      { value: 'env:read', label: 'View environments and their configuration' },
      { value: 'env:write', label: 'Create, update, and delete environments' },
    ],
  },
  {
    id: 'var',
    label: 'var',
    description: 'Manage configuration variables',
    scopes: [
      { value: 'var:read', label: 'View variables and their values' },
      { value: 'var:write', label: 'Create, update, and delete variables' },
    ],
  },
  {
    id: 'connection',
    label: 'connection',
    description: 'Manage external connections',
    scopes: [
      { value: 'connection:read', label: 'View connections' },
      { value: 'connection:write', label: 'Create, update, and delete connections' },
    ],
  },
  {
    id: 'state',
    label: 'state',
    description: 'Manage deployment state',
    scopes: [
      { value: 'state:read', label: 'View deployment state' },
      { value: 'state:write', label: 'Modify deployment state' },
      { value: 'state:admin', label: 'Administrative state operations' },
    ],
  },
  {
    id: 'token',
    label: 'token',
    description: 'Manage access tokens',
    scopes: [
      { value: 'token:read', label: 'View access tokens' },
      { value: 'token:write', label: 'Create and revoke access tokens' },
    ],
  },
  {
    id: 'user',
    label: 'user',
    description: 'User management',
    scopes: [
      { value: 'user:read', label: 'View user profiles' },
    ],
  },
];

export function allScopeValues(): string[] {
  return SCOPE_GROUPS.flatMap((g) => g.scopes.map((s) => s.value));
}

export function groupScopeValues(group: ScopeGroup): string[] {
  return group.scopes.map((s) => s.value);
}

export function isGroupFullySelected(group: ScopeGroup, selected: string[]): boolean {
  return group.scopes.every((s) => selected.includes(s.value));
}

export function isGroupPartiallySelected(group: ScopeGroup, selected: string[]): boolean {
  return group.scopes.some((s) => selected.includes(s.value)) && !isGroupFullySelected(group, selected);
}
