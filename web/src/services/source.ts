import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  sourceSchema,
  listSourcesSchema,
  listSourceVersionsSchema,
  testSourceResponseSchema,
  type Source,
  type SourceConfig,
  type ListSourcesResponse,
  type ListSourceVersionsResponse,
  type TestSourceResponse,
} from '@/types/source';

export interface ListSourcesParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListSourcesParams): Promise<ListSourcesResponse> {
  const { data, error } = await client.GET('/api/v1/sources', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list sources');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list sources response');
  }

  const parsed = listSourcesSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List sources parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list sources response');
  }

  return parsed.data;
}

export async function listAll(): Promise<Source[]> {
  const all: Source[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    all.push(...page.sources);
    page_token = page.next_page_token ?? undefined;
  } while (page_token);

  return all;
}

export async function get(sourceId: string): Promise<Source> {
  const { data, error } = await client.GET('/api/v1/sources/{source_id}', {
    params: { path: { source_id: sourceId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get source');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get source response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.source) {
    throw new Error('No source returned from API');
  }

  const parsed = sourceSchema.safeParse(raw.source);
  if (!parsed.success) {
    console.error('Get source parse error', parsed.error, raw.source);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid source in response');
  }

  return parsed.data;
}

export interface CreateSourceParams {
  name: string;
  description?: string;
  type: string;
  url: string;
  credential_id?: string | null;
  catalog?: boolean;
  source_config?: SourceConfig;
  labels?: Record<string, string>;
}

export async function create(params: CreateSourceParams): Promise<Source> {
  const { data, error } = await client.POST('/api/v1/sources', {
    body: {
      name: params.name,
      description: params.description,
      type: params.type as never,
      url: params.url,
      credential_id: params.credential_id,
      catalog: params.catalog,
      source_config: params.source_config as never,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create source');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create source response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.source) {
    throw new Error('No source returned from API');
  }

  const parsed = sourceSchema.safeParse(raw.source);
  if (!parsed.success) {
    console.error('Create source parse error', parsed.error, raw.source);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid source in response');
  }

  return parsed.data;
}

export interface UpdateSourceParams {
  id: string;
  name: string;
  updateName?: boolean;
  description?: string;
  url?: string;
  credential_id?: string | null;
  catalog?: boolean;
  source_config?: SourceConfig;
  labels?: Record<string, string>;
}

export async function update(params: UpdateSourceParams): Promise<Source> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.url !== undefined) fields.push('url');
  if (params.credential_id !== undefined) fields.push('credential_id');
  if (params.catalog !== undefined) fields.push('catalog');
  if (params.source_config !== undefined) fields.push('source_config');
  if (params.labels !== undefined) fields.push('labels');

  if (fields.length === 0) {
    return get(params.id);
  }

  const source: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) source.description = params.description;
  if (params.url !== undefined) source.url = params.url;
  if (params.credential_id !== undefined) source.credential_id = params.credential_id;
  if (params.catalog !== undefined) source.catalog = params.catalog;
  if (params.source_config !== undefined) source.source_config = params.source_config;
  if (params.labels !== undefined) source.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/sources/{source.id}', {
    params: { path: { 'source.id': params.id } },
    body: {
      source,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update source');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update source response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.source) {
    throw new Error('No source returned from API');
  }

  const parsed = sourceSchema.safeParse(raw.source);
  if (!parsed.success) {
    console.error('Update source parse error', parsed.error, raw.source);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid source in response');
  }

  return parsed.data;
}

export async function remove(sourceId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/sources/{source_id}', {
    params: { path: { source_id: sourceId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete source');
  }
}

export async function test(sourceId: string): Promise<TestSourceResponse> {
  const { data, error } = await client.POST('/api/v1/sources/{source_id}/test', {
    params: { path: { source_id: sourceId } },
    body: {},
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to test source');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in test source response');
  }

  const parsed = testSourceResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Test source parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid test source response');
  }

  return parsed.data;
}

export interface ListSourceVersionsParams {
  source_id: string;
  page_size?: number;
  page_token?: string;
}

export async function listVersions(
  params: ListSourceVersionsParams,
): Promise<ListSourceVersionsResponse> {
  const { data, error } = await client.GET('/api/v1/sources/{source_id}/versions', {
    params: {
      path: { source_id: params.source_id },
      query: { page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list source versions');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list source versions response');
  }

  const parsed = listSourceVersionsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List source versions parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list source versions response');
  }

  return parsed.data;
}
