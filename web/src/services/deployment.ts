import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  deploymentSchema,
  listDeploymentsSchema,
  revisionSchema,
  listRevisionsSchema,
  type Deployment,
  type Revision,
  type ListDeploymentsResponse,
  type ListRevisionsResponse,
} from '@/types/deployment';

export interface ListDeploymentsParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListDeploymentsParams): Promise<ListDeploymentsResponse> {
  const { data, error } = await client.GET('/api/v1/deployments', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list deployments');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list deployments response');
  }

  const parsed = listDeploymentsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List deployments parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list deployments response');
  }

  return parsed.data;
}

export async function get(deploymentId: string): Promise<Deployment> {
  const { data, error } = await client.GET('/api/v1/deployments/{deployment_id}', {
    params: { path: { deployment_id: deploymentId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get deployment');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get deployment response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.deployment) {
    throw new Error('No deployment returned from API');
  }

  const parsed = deploymentSchema.safeParse(raw.deployment);
  if (!parsed.success) {
    console.error('Get deployment parse error', parsed.error, raw.deployment);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid deployment in response');
  }

  return parsed.data;
}

export interface CreateDeploymentParams {
  application_id: string;
  environment_id: string;
  message?: string;
  destroy?: boolean;
  source_deployment_id?: string;
}

export async function create(params: CreateDeploymentParams): Promise<Deployment> {
  const { data, error } = await client.POST('/api/v1/deployments', {
    body: {
      application_id: params.application_id,
      environment_id: params.environment_id,
      message: params.message,
      destroy: params.destroy,
      source_deployment_id: params.source_deployment_id,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create deployment');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create deployment response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.deployment) {
    throw new Error('No deployment returned from API');
  }

  const parsed = deploymentSchema.safeParse(raw.deployment);
  if (!parsed.success) {
    console.error('Create deployment parse error', parsed.error, raw.deployment);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid deployment in response');
  }

  return parsed.data;
}

export async function apply(deploymentId: string, message?: string): Promise<Deployment> {
  const { data, error } = await client.POST('/api/v1/deployments/{deployment_id}/apply', {
    params: { path: { deployment_id: deploymentId } },
    body: { message },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to apply deployment');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in apply deployment response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.deployment) {
    throw new Error('No deployment returned from API');
  }

  const parsed = deploymentSchema.safeParse(raw.deployment);
  if (!parsed.success) {
    console.error('Apply deployment parse error', parsed.error, raw.deployment);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid deployment in response');
  }

  return parsed.data;
}

export async function cancel(deploymentId: string, reason?: string): Promise<Deployment> {
  const { data, error } = await client.POST('/api/v1/deployments/{deployment_id}/cancel', {
    params: { path: { deployment_id: deploymentId } },
    body: { reason },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to cancel deployment');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in cancel deployment response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.deployment) {
    throw new Error('No deployment returned from API');
  }

  const parsed = deploymentSchema.safeParse(raw.deployment);
  if (!parsed.success) {
    console.error('Cancel deployment parse error', parsed.error, raw.deployment);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid deployment in response');
  }

  return parsed.data;
}

// --- Revisions ---

export interface ListRevisionsParams {
  deployment_id: string;
  page_size?: number;
  page_token?: string;
}

export async function listRevisions(
  params: ListRevisionsParams,
): Promise<ListRevisionsResponse> {
  const { data, error } = await client.GET('/api/v1/deployments/{deployment_id}/revisions', {
    params: {
      path: { deployment_id: params.deployment_id },
      query: { page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list revisions');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list revisions response');
  }

  const parsed = listRevisionsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List revisions parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list revisions response');
  }

  return parsed.data;
}

export async function getRevision(
  deploymentId: string,
  revisionId: string,
): Promise<Revision> {
  const { data, error } = await client.GET(
    '/api/v1/deployments/{deployment_id}/revisions/{revision_id}',
    {
      params: {
        path: { deployment_id: deploymentId, revision_id: revisionId },
      },
    },
  );

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get revision');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get revision response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.revision) {
    throw new Error('No revision returned from API');
  }

  const parsed = revisionSchema.safeParse(raw.revision);
  if (!parsed.success) {
    console.error('Get revision parse error', parsed.error, raw.revision);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid revision in response');
  }

  return parsed.data;
}

export async function retryRevision(
  deploymentId: string,
  revisionId: string,
): Promise<Revision> {
  const { data, error } = await client.POST(
    '/api/v1/deployments/{deployment_id}/revisions/{revision_id}/retry',
    {
      params: {
        path: { deployment_id: deploymentId, revision_id: revisionId },
      },
      body: {},
    },
  );

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to retry revision');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in retry revision response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.revision) {
    throw new Error('No revision returned from API');
  }

  const parsed = revisionSchema.safeParse(raw.revision);
  if (!parsed.success) {
    console.error('Retry revision parse error', parsed.error, raw.revision);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid revision in response');
  }

  return parsed.data;
}
