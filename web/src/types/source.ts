import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

const helmConfigSchema = z.object({
  chart_name: z.string().optional(),
});

const terraformConfigSchema = z.object({
  namespace: z.string().optional(),
  module_name: z.string().optional(),
  system: z.string().optional(),
});

const sourceConfigSchema = z.object({
  helm: helmConfigSchema.optional(),
  terraform: terraformConfigSchema.optional(),
});

export const sourceSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  type: z.string().nullish(),
  url: z.string().nullish(),
  credential_id: z.string().nullish(),
  catalog: z.boolean().nullish(),
  source_config: sourceConfigSchema.nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  created_by: actorRefSchema.nullish(),
  last_test_status: z.string().nullish(),
  last_test_error: z.string().nullish(),
  last_tested_at: z.string().nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listSourcesSchema = z.object({
  sources: z.array(sourceSchema).default([]),
  next_page_token: z.string().nullish(),
});

export const sourceVersionSchema = z.object({
  version: z.string(),
  published_at: z.string().nullish(),
  description: z.string().nullish(),
});

export const listSourceVersionsSchema = z.object({
  versions: z.array(sourceVersionSchema).default([]),
  next_page_token: z.string().nullish(),
});

export const testSourceResponseSchema = z.object({
  status: z.string().nullish(),
  error: z.string().nullish(),
  source: sourceSchema.nullish(),
});

export type Source = z.infer<typeof sourceSchema>;
export type SourceConfig = z.infer<typeof sourceConfigSchema>;
export type SourceVersion = z.infer<typeof sourceVersionSchema>;
export type ListSourcesResponse = z.infer<typeof listSourcesSchema>;
export type ListSourceVersionsResponse = z.infer<typeof listSourceVersionsSchema>;
export type TestSourceResponse = z.infer<typeof testSourceResponseSchema>;
