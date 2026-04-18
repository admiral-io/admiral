import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  credentialSchema,
  listCredentialsSchema,
  type AuthConfig,
  type Credential,
  type ListCredentialsResponse,
} from '@/types/credential';

export interface ListCredentialsParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListCredentialsParams): Promise<ListCredentialsResponse> {
  const { data, error } = await client.GET('/api/v1/credentials', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list credentials');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list credentials response');
  }

  const parsed = listCredentialsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List credentials parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list credentials response');
  }

  return parsed.data;
}

export async function listAll(): Promise<Credential[]> {
  const all: Credential[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    all.push(...page.credentials);
    page_token = page.next_page_token ?? undefined;
  } while (page_token);

  return all;
}

export async function get(credentialId: string): Promise<Credential> {
  const { data, error } = await client.GET('/api/v1/credentials/{credential_id}', {
    params: { path: { credential_id: credentialId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get credential');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get credential response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.credential) {
    throw new Error('No credential returned from API');
  }

  const parsed = credentialSchema.safeParse(raw.credential);
  if (!parsed.success) {
    console.error('Get credential parse error', parsed.error, raw.credential);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid credential in response');
  }

  return parsed.data;
}

export interface CreateCredentialParams {
  name: string;
  description?: string;
  type: string;
  auth_config?: AuthConfig;
  labels?: Record<string, string>;
}

export async function create(params: CreateCredentialParams): Promise<Credential> {
  const { data, error } = await client.POST('/api/v1/credentials', {
    body: {
      name: params.name,
      description: params.description,
      type: params.type as never,
      auth_config: params.auth_config as never,
      labels: params.labels,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create credential');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create credential response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.credential) {
    throw new Error('No credential returned from API');
  }

  const parsed = credentialSchema.safeParse(raw.credential);
  if (!parsed.success) {
    console.error('Create credential parse error', parsed.error, raw.credential);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid credential in response');
  }

  return parsed.data;
}

export interface UpdateCredentialParams {
  id: string;
  name: string;
  updateName?: boolean;
  description?: string;
  auth_config?: AuthConfig;
  labels?: Record<string, string>;
}

export async function update(params: UpdateCredentialParams): Promise<Credential> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.auth_config !== undefined) fields.push('auth_config');
  if (params.labels !== undefined) fields.push('labels');

  if (fields.length === 0) {
    return get(params.id);
  }

  const credential: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) credential.description = params.description;
  if (params.auth_config !== undefined) credential.auth_config = params.auth_config;
  if (params.labels !== undefined) credential.labels = params.labels;

  const { data, error } = await client.PATCH('/api/v1/credentials/{credential.id}', {
    params: { path: { 'credential.id': params.id } },
    body: {
      credential,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update credential');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update credential response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.credential) {
    throw new Error('No credential returned from API');
  }

  const parsed = credentialSchema.safeParse(raw.credential);
  if (!parsed.success) {
    console.error('Update credential parse error', parsed.error, raw.credential);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid credential in response');
  }

  return parsed.data;
}

export async function remove(credentialId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/credentials/{credential_id}', {
    params: { path: { credential_id: credentialId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete credential');
  }
}
