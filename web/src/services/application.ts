import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  applicationSchema,
  listApplicationsSchema,
  type Application,
  type ListApplicationsResponse,
} from '@/types/application';

export interface ListApplicationsParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListApplicationsParams): Promise<ListApplicationsResponse> {
  const { data, error } = await client.GET('/api/v1/applications', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list applications');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list applications response');
  }

  const parsed = listApplicationsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List applications parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list applications response');
  }

  return parsed.data;
}

/** Paginate until all applications are loaded. */
export async function listAll(): Promise<Application[]> {
  const apps: Application[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    apps.push(...page.applications);
    page_token = page.next_page_token;
  } while (page_token);

  return apps;
}

export async function get(applicationId: string): Promise<Application> {
  const { data, error } = await client.GET('/api/v1/applications/{application_id}', {
    params: { path: { application_id: applicationId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get application');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get application response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.application) {
    throw new Error('No application returned from API');
  }

  const parsed = applicationSchema.safeParse(raw.application);
  if (!parsed.success) {
    console.error('Get application parse error', parsed.error, raw.application);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid application in response');
  }

  return parsed.data;
}

export interface CreateApplicationParams {
  name: string;
  description?: string;
  labels?: Record<string, string>;
}

export async function create(params: CreateApplicationParams): Promise<Application> {
  const { data, error } = await client.POST('/api/v1/applications', {
    body: {
      name: params.name,
      description: params.description ?? null,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create application');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create application response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.application) {
    throw new Error('No application returned from API');
  }

  const parsed = applicationSchema.safeParse(raw.application);
  if (!parsed.success) {
    console.error('Create application parse error', parsed.error, raw.application);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid application in response');
  }

  return parsed.data;
}

export interface UpdateApplicationParams {
  id: string;
  /** Always sent in the JSON body — required so gateway validation does not see an empty name. */
  name: string;
  /** When true, `name` is included in `update_mask` (rename). Omit when only description/labels change. */
  updateName?: boolean;
  description?: string;
  labels?: Record<string, string>;
}

export async function update(params: UpdateApplicationParams): Promise<Application> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.labels !== undefined) fields.push('labels');

  // Empty mask makes the server apply *all* mutable fields from the partial body (see
  // UpdateApplication in application.go), which can clear the name. Skip the PATCH.
  if (fields.length === 0) {
    return get(params.id);
  }

  const application: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) application.description = params.description;
  if (params.labels !== undefined) application.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/applications/{application.id}', {
    params: { path: { 'application.id': params.id } },
    body: {
      application,
      // google.protobuf.FieldMask JSON encoding (protojson, UseProtoNames): comma-separated paths
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update application');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update application response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.application) {
    throw new Error('No application returned from API');
  }

  const parsed = applicationSchema.safeParse(raw.application);
  if (!parsed.success) {
    console.error('Update application parse error', parsed.error, raw.application);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid application in response');
  }

  return parsed.data;
}

export async function remove(applicationId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/applications/{application_id}', {
    params: { path: { application_id: applicationId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete application');
  }
}
