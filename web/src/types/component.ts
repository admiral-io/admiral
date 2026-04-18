import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

const componentOutputSchema = z.object({
  name: z.string().optional(),
  value_template: z.string().optional(),
  description: z.string().optional(),
});

export const componentSchema = z.object({
  id: z.string(),
  application_id: z.string().nullish(),
  name: z.string(),
  description: z.string().nullish(),
  kind: z.string().nullish(),
  module_id: z.string().nullish(),
  version: z.string().nullish(),
  values_template: z.string().nullish(),
  depends_on: z.array(z.string()).nullish(),
  outputs: z.array(componentOutputSchema).nullish(),
  disabled: z.boolean().nullish(),
  has_override: z.boolean().nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listComponentsSchema = z.object({
  components: z.array(componentSchema).default([]),
  next_page_token: z.string().nullish(),
});

export const componentOverrideSchema = z.object({
  component_id: z.string().nullish(),
  environment_id: z.string().nullish(),
  disabled: z.boolean().nullish(),
  module_id: z.string().nullish(),
  version: z.string().nullish(),
  values_template: z.string().nullish(),
  depends_on: z.array(z.string()).nullish(),
  outputs: z.array(componentOutputSchema).nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listComponentOverridesSchema = z.object({
  overrides: z.array(componentOverrideSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type Component = z.infer<typeof componentSchema>;
export type ComponentOutput = z.infer<typeof componentOutputSchema>;
export type ComponentOverride = z.infer<typeof componentOverrideSchema>;
export type ListComponentsResponse = z.infer<typeof listComponentsSchema>;
export type ListComponentOverridesResponse = z.infer<typeof listComponentOverridesSchema>;
