import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  runSchema,
  listRunsSchema,
  revisionSchema,
  listRevisionsSchema,
  type Run,
  type Revision,
  type ListRunsResponse,
  type ListRevisionsResponse,
} from '@/types/run';

export interface ListRunsParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListRunsParams): Promise<ListRunsResponse> {
  const { data, error } = await client.GET('/api/v1/runs', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list runs');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list runs response');
  }

  const parsed = listRunsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List runs parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list runs response');
  }

  return parsed.data;
}

export async function get(runId: string): Promise<Run> {
  const { data, error } = await client.GET('/api/v1/runs/{run_id}', {
    params: { path: { run_id: runId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get run');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get run response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.run) {
    throw new Error('No run returned from API');
  }

  const parsed = runSchema.safeParse(raw.run);
  if (!parsed.success) {
    console.error('Get run parse error', parsed.error, raw.run);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid run in response');
  }

  return parsed.data;
}

export interface CreateRunParams {
  application_id: string;
  environment_id: string;
  message?: string;
  destroy?: boolean;
  source_run_id?: string;
  change_set_id?: string;
}

export async function create(params: CreateRunParams): Promise<Run> {
  const { data, error } = await client.POST('/api/v1/runs', {
    body: {
      application_id: params.application_id,
      environment_id: params.environment_id,
      message: params.message,
      destroy: params.destroy,
      source_run_id: params.source_run_id,
      change_set_id: params.change_set_id,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create run');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create run response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.run) {
    throw new Error('No run returned from API');
  }

  const parsed = runSchema.safeParse(raw.run);
  if (!parsed.success) {
    console.error('Create run parse error', parsed.error, raw.run);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid run in response');
  }

  return parsed.data;
}

export async function apply(runId: string, message?: string): Promise<Run> {
  const { data, error } = await client.POST('/api/v1/runs/{run_id}/apply', {
    params: { path: { run_id: runId } },
    body: { message },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to apply run');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in apply run response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.run) {
    throw new Error('No run returned from API');
  }

  const parsed = runSchema.safeParse(raw.run);
  if (!parsed.success) {
    console.error('Apply run parse error', parsed.error, raw.run);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid run in response');
  }

  return parsed.data;
}

export async function cancel(runId: string, reason?: string): Promise<Run> {
  const { data, error } = await client.POST('/api/v1/runs/{run_id}/cancel', {
    params: { path: { run_id: runId } },
    body: { reason },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to cancel run');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in cancel run response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.run) {
    throw new Error('No run returned from API');
  }

  const parsed = runSchema.safeParse(raw.run);
  if (!parsed.success) {
    console.error('Cancel run parse error', parsed.error, raw.run);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid run in response');
  }

  return parsed.data;
}

// --- Revisions ---

export interface ListRevisionsParams {
  run_id: string;
  page_size?: number;
  page_token?: string;
}

export async function listRevisions(
  params: ListRevisionsParams,
): Promise<ListRevisionsResponse> {
  const { data, error } = await client.GET('/api/v1/runs/{run_id}/revisions', {
    params: {
      path: { run_id: params.run_id },
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
  runId: string,
  revisionId: string,
): Promise<Revision> {
  const { data, error } = await client.GET(
    '/api/v1/runs/{run_id}/revisions/{revision_id}',
    {
      params: {
        path: { run_id: runId, revision_id: revisionId },
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
  runId: string,
  revisionId: string,
): Promise<Revision> {
  const { data, error } = await client.POST(
    '/api/v1/runs/{run_id}/revisions/{revision_id}/retry',
    {
      params: {
        path: { run_id: runId, revision_id: revisionId },
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
