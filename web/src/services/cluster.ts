import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import { accessTokenSchema, listTokensSchema } from '@/types/token';
import type { AccessToken, ListTokensResponse, CreateTokenResponse } from '@/types/token';
import { createTokenResponseSchema } from '@/types/token';
import {
  clusterSchema,
  listClustersSchema,
  createClusterResponseSchema,
  getClusterStatusResponseSchema,
  listWorkloadsSchema,
  type Cluster,
  type CreateClusterResponse,
  type GetClusterStatusResponse,
  type ListClustersResponse,
  type ListWorkloadsResponse,
} from '@/types/cluster';

export interface ListClustersParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListClustersParams): Promise<ListClustersResponse> {
  const { data, error } = await client.GET('/api/v1/clusters', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list clusters');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list clusters response');
  }

  const parsed = listClustersSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List clusters parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list clusters response');
  }

  return parsed.data;
}

export async function listAll(): Promise<Cluster[]> {
  const all: Cluster[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    all.push(...page.clusters);
    page_token = page.next_page_token ?? undefined;
  } while (page_token);

  return all;
}

export async function get(clusterId: string): Promise<Cluster> {
  const { data, error } = await client.GET('/api/v1/clusters/{cluster_id}', {
    params: { path: { cluster_id: clusterId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get cluster');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get cluster response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.cluster) {
    throw new Error('No cluster returned from API');
  }

  const parsed = clusterSchema.safeParse(raw.cluster);
  if (!parsed.success) {
    console.error('Get cluster parse error', parsed.error, raw.cluster);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid cluster in response');
  }

  return parsed.data;
}

export interface CreateClusterParams {
  name: string;
  description?: string;
  labels?: Record<string, string>;
}

export async function create(params: CreateClusterParams): Promise<CreateClusterResponse> {
  const { data, error } = await client.POST('/api/v1/clusters', {
    body: {
      name: params.name,
      description: params.description,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create cluster');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create cluster response');
  }

  const parsed = createClusterResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Create cluster parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid create cluster response');
  }

  return parsed.data;
}

export interface UpdateClusterParams {
  id: string;
  name: string;
  updateName?: boolean;
  description?: string;
  labels?: Record<string, string>;
}

export async function update(params: UpdateClusterParams): Promise<Cluster> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.labels !== undefined) fields.push('labels');

  if (fields.length === 0) {
    return get(params.id);
  }

  const cluster: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) cluster.description = params.description;
  if (params.labels !== undefined) cluster.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/clusters/{cluster.id}', {
    params: { path: { 'cluster.id': params.id } },
    body: {
      cluster,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update cluster');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update cluster response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.cluster) {
    throw new Error('No cluster returned from API');
  }

  const parsed = clusterSchema.safeParse(raw.cluster);
  if (!parsed.success) {
    console.error('Update cluster parse error', parsed.error, raw.cluster);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid cluster in response');
  }

  return parsed.data;
}

export async function remove(clusterId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/clusters/{cluster_id}', {
    params: { path: { cluster_id: clusterId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete cluster');
  }
}

// --- Cluster Status ---

export async function getStatus(clusterId: string): Promise<GetClusterStatusResponse> {
  const { data, error } = await client.GET('/api/v1/clusters/{cluster_id}/status', {
    params: { path: { cluster_id: clusterId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get cluster status');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get cluster status response');
  }

  const parsed = getClusterStatusResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Get cluster status parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid cluster status response');
  }

  return parsed.data;
}

// --- Cluster Workloads ---

export interface ListWorkloadsParams {
  cluster_id: string;
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function listWorkloads(params: ListWorkloadsParams): Promise<ListWorkloadsResponse> {
  const { data, error } = await client.GET('/api/v1/clusters/{cluster_id}/workloads', {
    params: {
      path: { cluster_id: params.cluster_id },
      query: { filter: params.filter, page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list workloads');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list workloads response');
  }

  const parsed = listWorkloadsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List workloads parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list workloads response');
  }

  return parsed.data;
}

// --- Cluster Tokens ---

export interface CreateClusterTokenParams {
  cluster_id: string;
  name: string;
  expires_at?: string;
}

export async function createToken(
  params: CreateClusterTokenParams,
): Promise<CreateTokenResponse> {
  const { data, error } = await client.POST('/api/v1/clusters/{cluster_id}/tokens', {
    params: { path: { cluster_id: params.cluster_id } },
    body: {
      name: params.name,
      ...(params.expires_at ? { expires_at: params.expires_at } : {}),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create cluster token');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create cluster token response');
  }

  const parsed = createTokenResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Create cluster token parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid create cluster token response');
  }

  return parsed.data;
}

export async function getToken(clusterId: string, tokenId: string): Promise<AccessToken> {
  const { data, error } = await client.GET('/api/v1/clusters/{cluster_id}/tokens/{token_id}', {
    params: { path: { cluster_id: clusterId, token_id: tokenId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get cluster token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return accessTokenSchema.parse(raw.access_token);
}

export interface ListClusterTokensParams {
  cluster_id: string;
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function listTokens(params: ListClusterTokensParams): Promise<ListTokensResponse> {
  const { data, error } = await client.GET('/api/v1/clusters/{cluster_id}/tokens', {
    params: {
      path: { cluster_id: params.cluster_id },
      query: { filter: params.filter, page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list cluster tokens');
  }

  const raw = data as Record<string, unknown>;
  return listTokensSchema.parse({
    tokens: raw.access_tokens ?? [],
    next_page_token: raw.next_page_token,
  });
}

export async function revokeToken(clusterId: string, tokenId: string): Promise<AccessToken> {
  const { data, error } = await client.POST(
    '/api/v1/clusters/{cluster_id}/tokens/{token_id}/revoke',
    {
      params: { path: { cluster_id: clusterId, token_id: tokenId } },
      body: {},
    },
  );

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to revoke cluster token');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.access_token) {
    throw new Error('No token returned from API');
  }

  return accessTokenSchema.parse(raw.access_token);
}
