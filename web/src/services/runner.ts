import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import { accessTokenSchema, listTokensSchema } from '@/types/token';
import type { AccessToken, ListTokensResponse, CreateTokenResponse } from '@/types/token';
import { createTokenResponseSchema } from '@/types/token';
import {
  runnerSchema,
  listRunnersSchema,
  createRunnerResponseSchema,
  getRunnerStatusResponseSchema,
  listRunnerJobsSchema,
  type Runner,
  type CreateRunnerResponse,
  type GetRunnerStatusResponse,
  type ListRunnersResponse,
  type ListRunnerJobsResponse,
} from '@/types/runner';

export interface ListRunnersParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListRunnersParams): Promise<ListRunnersResponse> {
  const { data, error } = await client.GET('/api/v1/runners', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list runners');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list runners response');
  }

  const parsed = listRunnersSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List runners parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list runners response');
  }

  return parsed.data;
}

export async function listAll(): Promise<Runner[]> {
  const all: Runner[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    all.push(...page.runners);
    page_token = page.next_page_token ?? undefined;
  } while (page_token);

  return all;
}

export async function get(runnerId: string): Promise<Runner> {
  const { data, error } = await client.GET('/api/v1/runners/{runner_id}', {
    params: { path: { runner_id: runnerId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get runner');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get runner response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.runner) {
    throw new Error('No runner returned from API');
  }

  const parsed = runnerSchema.safeParse(raw.runner);
  if (!parsed.success) {
    console.error('Get runner parse error', parsed.error, raw.runner);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid runner in response');
  }

  return parsed.data;
}

export interface CreateRunnerParams {
  name: string;
  description?: string;
  kind?: string;
  labels?: Record<string, string>;
}

export async function create(params: CreateRunnerParams): Promise<CreateRunnerResponse> {
  const { data, error } = await client.POST('/api/v1/runners', {
    body: {
      name: params.name,
      description: params.description,
      kind: params.kind as never,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create runner');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create runner response');
  }

  const parsed = createRunnerResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Create runner parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid create runner response');
  }

  return parsed.data;
}

export interface UpdateRunnerParams {
  id: string;
  name: string;
  updateName?: boolean;
  description?: string;
  kind?: string;
  labels?: Record<string, string>;
}

export async function update(params: UpdateRunnerParams): Promise<Runner> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.kind !== undefined) fields.push('kind');
  if (params.labels !== undefined) fields.push('labels');

  if (fields.length === 0) {
    return get(params.id);
  }

  const runner: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) runner.description = params.description;
  if (params.kind !== undefined) runner.kind = params.kind;
  if (params.labels !== undefined) runner.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/runners/{runner.id}', {
    params: { path: { 'runner.id': params.id } },
    body: {
      runner,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update runner');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update runner response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.runner) {
    throw new Error('No runner returned from API');
  }

  const parsed = runnerSchema.safeParse(raw.runner);
  if (!parsed.success) {
    console.error('Update runner parse error', parsed.error, raw.runner);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid runner in response');
  }

  return parsed.data;
}

export async function remove(runnerId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/runners/{runner_id}', {
    params: { path: { runner_id: runnerId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete runner');
  }
}

// --- Runner Status ---

export async function getStatus(runnerId: string): Promise<GetRunnerStatusResponse> {
  const { data, error } = await client.GET('/api/v1/runners/{runner_id}/status', {
    params: { path: { runner_id: runnerId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get runner status');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get runner status response');
  }

  const parsed = getRunnerStatusResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Get runner status parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid runner status response');
  }

  return parsed.data;
}

// --- Runner Jobs ---

export interface ListRunnerJobsParams {
  runner_id: string;
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function listJobs(params: ListRunnerJobsParams): Promise<ListRunnerJobsResponse> {
  const { data, error } = await client.GET('/api/v1/runners/{runner_id}/jobs', {
    params: {
      path: { runner_id: params.runner_id },
      query: { filter: params.filter, page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list runner jobs');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list runner jobs response');
  }

  const parsed = listRunnerJobsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List runner jobs parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list runner jobs response');
  }

  return parsed.data;
}

// --- Runner Tokens ---

export interface CreateRunnerTokenParams {
  runner_id: string;
  name: string;
  expires_at?: string;
}

export async function createToken(params: CreateRunnerTokenParams): Promise<CreateTokenResponse> {
  const { data, error } = await client.POST('/api/v1/runners/{runner_id}/tokens', {
    params: { path: { runner_id: params.runner_id } },
    body: {
      name: params.name,
      ...(params.expires_at ? { expires_at: params.expires_at } : {}),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create runner token');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create runner token response');
  }

  const parsed = createTokenResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Create runner token parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid create runner token response');
  }

  return parsed.data;
}

export async function getToken(tokenId: string): Promise<AccessToken> {
  const { data, error } = await client.GET('/api/v1/runners/tokens/{token_id}', {
    params: { path: { token_id: tokenId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get runner token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return accessTokenSchema.parse(raw.access_token);
}

export interface ListRunnerTokensParams {
  runner_id: string;
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function listTokens(params: ListRunnerTokensParams): Promise<ListTokensResponse> {
  const { data, error } = await client.GET('/api/v1/runners/{runner_id}/tokens', {
    params: {
      path: { runner_id: params.runner_id },
      query: { filter: params.filter, page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list runner tokens');
  }

  const raw = data as Record<string, unknown>;
  return listTokensSchema.parse({
    tokens: raw.access_tokens ?? [],
    next_page_token: raw.next_page_token,
  });
}

export async function revokeToken(tokenId: string): Promise<AccessToken> {
  const { data, error } = await client.POST(
    '/api/v1/runners/tokens/{token_id}/revoke',
    {
      params: { path: { token_id: tokenId } },
      body: {},
    },
  );

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to revoke runner token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return accessTokenSchema.parse(raw.access_token);
}
