import type { AccessToken } from '@/types/token';

const TOKEN_NAME_PATTERN = /^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$/;

export function validateTokenName(value: string): string | undefined {
  if (!value) return undefined;
  if (TOKEN_NAME_PATTERN.test(value)) return undefined;
  return 'Must start with a lowercase letter, use only lowercase letters, numbers, and hyphens, and end with a letter or number (max 63 characters). Example: ci-deploy-token';
}

export function formatTokenDate(iso: string | null | undefined): string {
  if (!iso) return '\u2014';
  try {
    return new Intl.DateTimeFormat('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

export function isTokenExpired(token: AccessToken): boolean {
  if (!token.expires_at) return false;
  return new Date(token.expires_at) < new Date();
}

export function resolveTokenStatus(token: AccessToken): 'active' | 'revoked' | 'expired' {
  if (token.status === 'revoked' || token.revoked_at) return 'revoked';
  if (isTokenExpired(token)) return 'expired';
  return 'active';
}
