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

  return listApplicationsSchema.parse(data);
}

export async function get(applicationId: string): Promise<Application> {
  const { data, error } = await client.GET('/api/v1/applications/{application_id}', {
    params: { path: { application_id: applicationId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get application');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.application) {
    throw new Error('No application returned from API');
  }

  return applicationSchema.parse(raw.application);
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
      description: params.description,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create application');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.application) {
    throw new Error('No application returned from API');
  }

  return applicationSchema.parse(raw.application);
}

export interface UpdateApplicationParams {
  id: string;
  name?: string;
  description?: string;
  labels?: Record<string, string>;
}

export async function update(params: UpdateApplicationParams): Promise<Application> {
  const fields: string[] = [];
  if (params.name !== undefined) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.labels !== undefined) fields.push('labels');

  const application: Record<string, unknown> = { id: params.id };
  if (params.name !== undefined) application.name = params.name;
  if (params.description !== undefined) application.description = params.description;
  if (params.labels !== undefined) application.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/applications/{application.id}', {
    params: { path: { 'application.id': params.id } },
    body: {
      application,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update application');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.application) {
    throw new Error('No application returned from API');
  }

  return applicationSchema.parse(raw.application);
}

export async function remove(applicationId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/applications/{application_id}', {
    params: { path: { application_id: applicationId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete application');
  }
}
