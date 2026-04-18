import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  variableSchema,
  listVariablesSchema,
  type Variable,
  type ListVariablesResponse,
} from '@/types/variable';

export interface ListVariablesParams {
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListVariablesParams): Promise<ListVariablesResponse> {
  const { data, error } = await client.GET('/api/v1/variables', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list variables');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list variables response');
  }

  const parsed = listVariablesSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List variables parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list variables response');
  }

  return parsed.data;
}

export async function listAll(): Promise<Variable[]> {
  const all: Variable[] = [];
  let page_token: string | undefined;
  do {
    const page = await list({ page_size: 100, page_token });
    all.push(...page.variables);
    page_token = page.next_page_token ?? undefined;
  } while (page_token);

  return all;
}

export async function get(variableId: string): Promise<Variable> {
  const { data, error } = await client.GET('/api/v1/variables/{variable_id}', {
    params: { path: { variable_id: variableId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get variable');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get variable response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.variable) {
    throw new Error('No variable returned from API');
  }

  const parsed = variableSchema.safeParse(raw.variable);
  if (!parsed.success) {
    console.error('Get variable parse error', parsed.error, raw.variable);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid variable in response');
  }

  return parsed.data;
}

export interface CreateVariableParams {
  key: string;
  value: string;
  sensitive?: boolean;
  type?: string;
  description?: string;
  application_id?: string;
  environment_id?: string;
}

export async function create(params: CreateVariableParams): Promise<Variable> {
  const { data, error } = await client.POST('/api/v1/variables', {
    body: {
      key: params.key,
      value: params.value,
      sensitive: params.sensitive,
      type: params.type as never,
      description: params.description,
      application_id: params.application_id,
      environment_id: params.environment_id,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create variable');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create variable response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.variable) {
    throw new Error('No variable returned from API');
  }

  const parsed = variableSchema.safeParse(raw.variable);
  if (!parsed.success) {
    console.error('Create variable parse error', parsed.error, raw.variable);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid variable in response');
  }

  return parsed.data;
}

export interface UpdateVariableParams {
  id: string;
  key?: string;
  value?: string;
  sensitive?: boolean;
  type?: string;
  description?: string;
}

export async function update(params: UpdateVariableParams): Promise<Variable> {
  const fields: string[] = [];
  if (params.key !== undefined) fields.push('key');
  if (params.value !== undefined) fields.push('value');
  if (params.sensitive !== undefined) fields.push('sensitive');
  if (params.type !== undefined) fields.push('type');
  if (params.description !== undefined) fields.push('description');

  if (fields.length === 0) {
    return get(params.id);
  }

  const variable: Record<string, unknown> = { id: params.id };
  if (params.key !== undefined) variable.key = params.key;
  if (params.value !== undefined) variable.value = params.value;
  if (params.sensitive !== undefined) variable.sensitive = params.sensitive;
  if (params.type !== undefined) variable.type = params.type;
  if (params.description !== undefined) variable.description = params.description;

  const { data, error } = await client.PATCH('/api/v1/variables/{variable.id}', {
    params: { path: { 'variable.id': params.id } },
    body: {
      variable,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update variable');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update variable response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.variable) {
    throw new Error('No variable returned from API');
  }

  const parsed = variableSchema.safeParse(raw.variable);
  if (!parsed.success) {
    console.error('Update variable parse error', parsed.error, raw.variable);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid variable in response');
  }

  return parsed.data;
}

export async function remove(variableId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/variables/{variable_id}', {
    params: { path: { variable_id: variableId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete variable');
  }
}
