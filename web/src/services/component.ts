import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import {
  componentSchema,
  listComponentsSchema,
  componentOverrideSchema,
  listComponentOverridesSchema,
  type Component,
  type ComponentOutput,
  type ComponentOverride,
  type ListComponentsResponse,
  type ListComponentOverridesResponse,
} from '@/types/component';

export interface ListComponentsParams {
  application_id?: string;
  environment_id?: string;
  filter?: string;
  page_size?: number;
  page_token?: string;
}

export async function list(params?: ListComponentsParams): Promise<ListComponentsResponse> {
  const { data, error } = await client.GET('/api/v1/components', {
    params: { query: params },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list components');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list components response');
  }

  const parsed = listComponentsSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List components parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list components response');
  }

  return parsed.data;
}

export async function get(componentId: string): Promise<Component> {
  const { data, error } = await client.GET('/api/v1/components/{component_id}', {
    params: { path: { component_id: componentId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get component');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get component response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.component) {
    throw new Error('No component returned from API');
  }

  const parsed = componentSchema.safeParse(raw.component);
  if (!parsed.success) {
    console.error('Get component parse error', parsed.error, raw.component);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid component in response');
  }

  return parsed.data;
}

export interface CreateComponentParams {
  application_id: string;
  name: string;
  description?: string;
  module_id: string;
  version?: string;
  values_template?: string;
  depends_on?: string[];
  outputs?: ComponentOutput[];
}

export async function create(params: CreateComponentParams): Promise<Component> {
  const { data, error } = await client.POST('/api/v1/components', {
    body: {
      application_id: params.application_id,
      name: params.name,
      description: params.description,
      module_id: params.module_id,
      version: params.version,
      values_template: params.values_template,
      depends_on: params.depends_on,
      outputs: params.outputs,
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to create component');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in create component response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.component) {
    throw new Error('No component returned from API');
  }

  const parsed = componentSchema.safeParse(raw.component);
  if (!parsed.success) {
    console.error('Create component parse error', parsed.error, raw.component);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid component in response');
  }

  return parsed.data;
}

export interface UpdateComponentParams {
  id: string;
  name: string;
  updateName?: boolean;
  description?: string;
  module_id?: string;
  version?: string;
  values_template?: string;
  depends_on?: string[];
  outputs?: ComponentOutput[];
}

export async function update(params: UpdateComponentParams): Promise<Component> {
  const fields: string[] = [];
  if (params.updateName) fields.push('name');
  if (params.description !== undefined) fields.push('description');
  if (params.module_id !== undefined) fields.push('module_id');
  if (params.version !== undefined) fields.push('version');
  if (params.values_template !== undefined) fields.push('values_template');
  if (params.depends_on !== undefined) fields.push('depends_on');
  if (params.outputs !== undefined) fields.push('outputs');

  if (fields.length === 0) {
    return get(params.id);
  }

  const component: Record<string, unknown> = {
    id: params.id,
    name: params.name,
  };
  if (params.description !== undefined) component.description = params.description;
  if (params.module_id !== undefined) component.module_id = params.module_id;
  if (params.version !== undefined) component.version = params.version;
  if (params.values_template !== undefined) component.values_template = params.values_template;
  if (params.depends_on !== undefined) component.depends_on = params.depends_on;
  if (params.outputs !== undefined) component.outputs = params.outputs;

  const { data, error } = await client.PATCH('/api/v1/components/{component.id}', {
    params: { path: { 'component.id': params.id } },
    body: {
      component,
      update_mask: fields.join(','),
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to update component');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in update component response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.component) {
    throw new Error('No component returned from API');
  }

  const parsed = componentSchema.safeParse(raw.component);
  if (!parsed.success) {
    console.error('Update component parse error', parsed.error, raw.component);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid component in response');
  }

  return parsed.data;
}

export async function remove(componentId: string): Promise<void> {
  const { error } = await client.DELETE('/api/v1/components/{component_id}', {
    params: { path: { component_id: componentId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete component');
  }
}

// --- Component Overrides ---

export interface SetComponentOverrideParams {
  component_id: string;
  environment_id: string;
  disabled?: boolean;
  module_id?: string | null;
  version?: string | null;
  values_template?: string | null;
  depends_on?: string[];
  outputs?: ComponentOutput[];
}

export async function setOverride(
  params: SetComponentOverrideParams,
): Promise<ComponentOverride> {
  const { data, error } = await client.PUT(
    '/api/v1/components/{component_id}/overrides/{environment_id}',
    {
      params: {
        path: {
          component_id: params.component_id,
          environment_id: params.environment_id,
        },
      },
      body: {
        disabled: params.disabled,
        module_id: params.module_id,
        version: params.version,
        values_template: params.values_template,
        depends_on: params.depends_on,
        outputs: params.outputs,
      },
    },
  );

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to set component override');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in set component override response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.override) {
    throw new Error('No override returned from API');
  }

  const parsed = componentOverrideSchema.safeParse(raw.override);
  if (!parsed.success) {
    console.error('Set component override parse error', parsed.error, raw.override);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid override in response');
  }

  return parsed.data;
}

export async function getOverride(
  componentId: string,
  environmentId: string,
): Promise<ComponentOverride> {
  const { data, error } = await client.GET(
    '/api/v1/components/{component_id}/overrides/{environment_id}',
    {
      params: {
        path: { component_id: componentId, environment_id: environmentId },
      },
    },
  );

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to get component override');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in get component override response');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.override) {
    throw new Error('No override returned from API');
  }

  const parsed = componentOverrideSchema.safeParse(raw.override);
  if (!parsed.success) {
    console.error('Get component override parse error', parsed.error, raw.override);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid override in response');
  }

  return parsed.data;
}

export interface ListComponentOverridesParams {
  component_id: string;
  page_size?: number;
  page_token?: string;
}

export async function listOverrides(
  params: ListComponentOverridesParams,
): Promise<ListComponentOverridesResponse> {
  const { data, error } = await client.GET('/api/v1/components/{component_id}/overrides', {
    params: {
      path: { component_id: params.component_id },
      query: { page_size: params.page_size, page_token: params.page_token },
    },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to list component overrides');
  }

  if (data === undefined || data === null) {
    throw new Error('No data in list component overrides response');
  }

  const parsed = listComponentOverridesSchema.safeParse(data);
  if (!parsed.success) {
    console.error('List component overrides parse error', parsed.error, data);
    throw new Error(parsed.error.issues[0]?.message ?? 'Invalid list overrides response');
  }

  return parsed.data;
}

export async function deleteOverride(
  componentId: string,
  environmentId: string,
): Promise<void> {
  const { error } = await client.DELETE(
    '/api/v1/components/{component_id}/overrides/{environment_id}',
    {
      params: {
        path: { component_id: componentId, environment_id: environmentId },
      },
    },
  );

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to delete component override');
  }
}
