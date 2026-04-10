import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  accessTokenSchema,
  listTokensSchema,
  createTokenResponseSchema,
  type AccessToken,
  type ListTokensResponse,
  type CreateTokenResponse,
} from '@/types/token';

export interface ListTokensParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListTokensParams): Promise<ListTokensResponse> {
  const { data, error } = await client.GET('/api/v1/user/tokens', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list tokens');
  }

  const raw = data as Record<string, unknown>;
  return listTokensSchema.parse({
    tokens: raw.access_tokens ?? [],
    next_page_token: raw.next_page_token,
  });
}

export async function get(tokenId: string): Promise<AccessToken> {
  const { data, error } = await client.GET('/api/v1/user/tokens/{token_id}', {
    params: { path: { token_id: tokenId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to fetch token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return accessTokenSchema.parse(raw.access_token);
}

export interface CreateTokenParams {
  name: string;
  scopes: string[];
  expiresAt?: string;
}

export async function create(params: CreateTokenParams): Promise<CreateTokenResponse> {
  const { data, error } = await client.POST('/api/v1/user/tokens', {
    body: {
      name: params.name,
      scopes: params.scopes,
      ...(params.expiresAt ? { expires_at: params.expiresAt } : {}),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return createTokenResponseSchema.parse(raw);
}

export interface UpdateTokenParams {
  tokenId: string;
  name?: string;
  scopes?: string[];
}

export async function update(params: UpdateTokenParams): Promise<AccessToken> {
  const { data, error } = await client.PATCH('/api/v1/user/tokens/{token_id}', {
    params: { path: { token_id: params.tokenId } },
    body: {
      name: params.name,
      scopes: params.scopes,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return accessTokenSchema.parse(raw.access_token);
}

export async function revoke(tokenId: string): Promise<AccessToken> {
  const { data, error } = await client.POST('/api/v1/user/tokens/{token_id}/revoke', {
    params: { path: { token_id: tokenId } },
    body: {},
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to revoke token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return accessTokenSchema.parse(raw.access_token);
}
