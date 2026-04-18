import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  moduleSchema,
  listModulesSchema,
  resolveModuleResponseSchema,
  type Module,
  type ListModulesResponse,
  type ResolveModuleResponse,
} from '@/types/module';

export interface ListModulesParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListModulesParams): Promise<ListModulesResponse> {
  const { data, error } = await client.GET('/api/v1/modules', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list modules');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list modules response');
  }

  const parsed = listModulesSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List modules parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list modules response');
  }

  return parsed.data;
}

export async function listAll(): Promise<Module[]> {
  const all: Module[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    all.push(...page.modules);
    page_token = page.next_page_token ?? undefined;
  } while (page_token);

  return all;
}

export async function get(moduleId: string): Promise<Module> {
  const { data, error } = await client.GET('/api/v1/modules/{module_id}', {
    params: { path: { module_id: moduleId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get module');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get module response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.module) {
    throw new Error('No module returned from API');
  }

  const parsed = moduleSchema.safeParse(raw.module);
  if (!parsed.success) {
    console.error('Get module parse error', parsed.error, raw.module);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid module in response');
  }

  return parsed.data;
}

export interface CreateModuleParams {
  name: string;
  description?: string;
  type: string;
  source_id: string;
  ref: string;
  root?: string;
  path: string;
  labels?: Record<string, string>;
}

export async function create(params: CreateModuleParams): Promise<Module> {
  const { data, error } = await client.POST('/api/v1/modules', {
    body: {
      name: params.name,
      description: params.description,
      type: params.type as never,
      source_id: params.source_id,
      ref: params.ref,
      root: params.root,
      path: params.path,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create module');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create module response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.module) {
    throw new Error('No module returned from API');
  }

  const parsed = moduleSchema.safeParse(raw.module);
  if (!parsed.success) {
    console.error('Create module parse error', parsed.error, raw.module);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid module in response');
  }

  return parsed.data;
}

export interface UpdateModuleParams {
  id: string;
  name: string;
  updateName?: boolean;
  description?: string;
  ref?: string;
  root?: string;
  path?: string;
  labels?: Record<string, string>;
}

export async function update(params: UpdateModuleParams): Promise<Module> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.ref !== undefined) fields.push('ref');
  if (params.root !== undefined) fields.push('root');
  if (params.path !== undefined) fields.push('path');
  if (params.labels !== undefined) fields.push('labels');

  if (fields.length === 0) {
    return get(params.id);
  }

  const module: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) module.description = params.description;
  if (params.ref !== undefined) module.ref = params.ref;
  if (params.root !== undefined) module.root = params.root;
  if (params.path !== undefined) module.path = params.path;
  if (params.labels !== undefined) module.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/modules/{module.id}', {
    params: { path: { 'module.id': params.id } },
    body: {
      module,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update module');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update module response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.module) {
    throw new Error('No module returned from API');
  }

  const parsed = moduleSchema.safeParse(raw.module);
  if (!parsed.success) {
    console.error('Update module parse error', parsed.error, raw.module);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid module in response');
  }

  return parsed.data;
}

export async function remove(moduleId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/modules/{module_id}', {
    params: { path: { module_id: moduleId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete module');
  }
}

export async function resolve(
  moduleId: string,
  refOverride?: string,
): Promise<ResolveModuleResponse> {
  const { data, error } = await client.POST('/api/v1/modules/{module_id}/resolve', {
    params: { path: { module_id: moduleId } },
    body: { ref_override: refOverride },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to resolve module');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in resolve module response');
  }

  const parsed = resolveModuleResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Resolve module parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid resolve module response');
  }

  return parsed.data;
}
