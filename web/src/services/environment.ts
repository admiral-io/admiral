import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  environmentSchema,
  listEnvironmentsSchema,
  type Environment,
  type InfrastructureTarget,
  type ListEnvironmentsResponse,
  type WorkloadTarget,
} from '@/types/environment';

export interface ListEnvironmentsParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListEnvironmentsParams): Promise<ListEnvironmentsResponse> {
  const { data, error } = await client.GET('/api/v1/environments', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list environments');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list environments response');
  }

  const parsed = listEnvironmentsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List environments parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list environments response');
  }

  return parsed.data;
}

/** Paginate until all environments for the tenant are loaded. */
export async function listAll(): Promise<Environment[]> {
  const all: Environment[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    all.push(...page.environments);
    page_token = page.next_page_token ?? undefined;
  } while (page_token);

  return all;
}

export async function get(environmentId: string): Promise<Environment> {
  const { data, error } = await client.GET('/api/v1/environments/{environment_id}', {
    params: { path: { environment_id: environmentId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get environment');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get environment response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.environment) {
    throw new Error('No environment returned from API');
  }

  const parsed = environmentSchema.safeParse(raw.environment);
  if (!parsed.success) {
    console.error('Get environment parse error', parsed.error, raw.environment);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid environment in response');
  }

  return parsed.data;
}

export interface CreateEnvironmentParams {
  application_id: string;
  name: string;
  description?: string;
  workload_targets?: WorkloadTarget[];
  infrastructure_targets?: InfrastructureTarget[];
  labels?: Record<string, string>;
}

export async function create(params: CreateEnvironmentParams): Promise<Environment> {
  const { data, error } = await client.POST('/api/v1/environments', {
    body: {
      application_id: params.application_id,
      name: params.name,
      description: params.description,
      workload_targets: params.workload_targets,
      infrastructure_targets: params.infrastructure_targets,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create environment');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create environment response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.environment) {
    throw new Error('No environment returned from API');
  }

  const parsed = environmentSchema.safeParse(raw.environment);
  if (!parsed.success) {
    console.error('Create environment parse error', parsed.error, raw.environment);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid environment in response');
  }

  return parsed.data;
}

export interface UpdateEnvironmentParams {
  id: string;
  name: string;
  updateName?: boolean;
  description?: string;
  workload_targets?: WorkloadTarget[];
  infrastructure_targets?: InfrastructureTarget[];
  labels?: Record<string, string>;
}

export async function update(params: UpdateEnvironmentParams): Promise<Environment> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.workload_targets !== undefined) fields.push('workload_targets');
  if (params.infrastructure_targets !== undefined) fields.push('infrastructure_targets');
  if (params.labels !== undefined) fields.push('labels');

  if (fields.length === 0) {
    return get(params.id);
  }

  const environment: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) environment.description = params.description;
  if (params.workload_targets !== undefined) environment.workload_targets = params.workload_targets;
  if (params.infrastructure_targets !== undefined)
    environment.infrastructure_targets = params.infrastructure_targets;
  if (params.labels !== undefined) environment.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/environments/{environment.id}', {
    params: { path: { 'environment.id': params.id } },
    body: {
      environment,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update environment');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update environment response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.environment) {
    throw new Error('No environment returned from API');
  }

  const parsed = environmentSchema.safeParse(raw.environment);
  if (!parsed.success) {
    console.error('Update environment parse error', parsed.error, raw.environment);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid environment in response');
  }

  return parsed.data;
}

export async function remove(environmentId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/environments/{environment_id}', {
    params: { path: { environment_id: environmentId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete environment');
  }
}

export function countByApplicationId(envs: Environment[]): Map<string, number> {
  const map = new Map<string, number>();
  for (const e of envs) {
    map.set(e.application_id, (map.get(e.application_id) ?? 0) + 1);
  }
  return map;
}

/** Environment display names per application, sorted alphabetically. */
export function environmentNamesByApplicationId(envs: Environment[]): Map<string, string[]> {
  const map = new Map<string, string[]>();
  for (const e of envs) {
    const n = e.name?.trim();
    if (!n) continue;
    const list = map.get(e.application_id) ?? [];
    list.push(n);
    map.set(e.application_id, list);
  }
  for (const [, names] of map) {
    names.sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }));
  }
  return map;
}
