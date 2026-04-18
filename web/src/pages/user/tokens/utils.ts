import type { AccessToken } from '@/types/token';
import { validateResourceName } from '@/utils/validation';

export const validateTokenName = validateResourceName;

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
