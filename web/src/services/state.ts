import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  stateSchema,
  listStatesSchema,
  listStateVersionsSchema,
  getStateVersionResponseSchema,
  type State,
  type ListStatesResponse,
  type ListStateVersionsResponse,
  type GetStateVersionResponse,
} from '@/types/state';

export interface ListStatesParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListStatesParams): Promise<ListStatesResponse> {
  const { data, error } = await client.GET('/api/v1/states', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list states');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list states response');
  }

  const parsed = listStatesSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List states parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list states response');
  }

  return parsed.data;
}

export async function get(stateId: string): Promise<State> {
  const { data, error } = await client.GET('/api/v1/states/{state_id}', {
    params: { path: { state_id: stateId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get state');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get state response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.state) {
    throw new Error('No state returned from API');
  }

  const parsed = stateSchema.safeParse(raw.state);
  if (!parsed.success) {
    console.error('Get state parse error', parsed.error, raw.state);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid state in response');
  }

  return parsed.data;
}

export async function remove(stateId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/states/{state_id}', {
    params: { path: { state_id: stateId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete state');
  }
}

export async function forceUnlock(stateId: string, reason?: string): Promise<void> {
  const { error } = await client.POST('/api/v1/states/{state_id}/force-unlock', {
    params: { path: { state_id: stateId } },
    body: { reason },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to force unlock state');
  }
}

// --- State Versions ---

export interface ListStateVersionsParams {
  state_id: string;
  page_size?: number;
  page_token?: string;
}

export async function listVersions(
  params: ListStateVersionsParams,
): Promise<ListStateVersionsResponse> {
  const { data, error } = await client.GET('/api/v1/states/{state_id}/versions', {
    params: {
      path: { state_id: params.state_id },
      query: { page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list state versions');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list state versions response');
  }

  const parsed = listStateVersionsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List state versions parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list state versions response');
  }

  return parsed.data;
}

export async function getVersion(
  stateId: string,
  serial: number | string,
): Promise<GetStateVersionResponse> {
  const { data, error } = await client.GET('/api/v1/states/{state_id}/versions/{serial}', {
    params: { path: { state_id: stateId, serial: Number(serial) } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get state version');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get state version response');
  }

  const parsed = getStateVersionResponseSchema.safeParse(data);
  if (!parsed.success) {
    console.error('Get state version parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid state version response');
  }

  return parsed.data;
}
