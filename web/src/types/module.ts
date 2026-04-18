import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

export const moduleSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  type: z.string().nullish(),
  source_id: z.string().nullish(),
  ref: z.string().nullish(),
  root: z.string().nullish(),
  path: z.string().nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listModulesSchema = z.object({
  modules: z.array(moduleSchema).default([]),
  next_page_token: z.string().nullish(),
});

export const resolveModuleResponseSchema = z.object({
  revision: z.string().nullish(),
  digest: z.string().nullish(),
  module: moduleSchema.nullish(),
});

export type Module = z.infer<typeof moduleSchema>;
export type ListModulesResponse = z.infer<typeof listModulesSchema>;
export type ResolveModuleResponse = z.infer<typeof resolveModuleResponseSchema>;
