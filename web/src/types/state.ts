import { z } from 'zod';

const stateLockSchema = z.object({
  lock_id: z.string().nullish(),
  operation: z.string().nullish(),
  who: z.string().nullish(),
  version: z.string().nullish(),
  acquired_at: z.string().nullish(),
});

export const stateSummarySchema = z.object({
  id: z.string(),
  component_id: z.string().nullish(),
  environment_id: z.string().nullish(),
  serial: z.union([z.number(), z.string()]).nullish(),
  md5: z.string().nullish(),
  lineage: z.string().nullish(),
  size_bytes: z.union([z.number(), z.string()]).nullish(),
  lock: stateLockSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const stateSchema = z.object({
  id: z.string(),
  component_id: z.string().nullish(),
  environment_id: z.string().nullish(),
  serial: z.union([z.number(), z.string()]).nullish(),
  data: z.string().nullish(),
  md5: z.string().nullish(),
  lineage: z.string().nullish(),
  lock: stateLockSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listStatesSchema = z.object({
  states: z.array(stateSummarySchema).default([]),
  next_page_token: z.string().nullish(),
});

export const stateVersionSchema = z.object({
  serial: z.union([z.number(), z.string()]).nullish(),
  md5: z.string().nullish(),
  lineage: z.string().nullish(),
  size_bytes: z.union([z.number(), z.string()]).nullish(),
  job_id: z.string().nullish(),
  created_at: z.string().nullish(),
});

export const listStateVersionsSchema = z.object({
  versions: z.array(stateVersionSchema).default([]),
  next_page_token: z.string().nullish(),
});

export const getStateVersionResponseSchema = z.object({
  version: stateVersionSchema.nullish(),
  data: z.string().nullish(),
});

export type StateLock = z.infer<typeof stateLockSchema>;
export type StateSummary = z.infer<typeof stateSummarySchema>;
export type State = z.infer<typeof stateSchema>;
export type StateVersion = z.infer<typeof stateVersionSchema>;
export type ListStatesResponse = z.infer<typeof listStatesSchema>;
export type ListStateVersionsResponse = z.infer<typeof listStateVersionsSchema>;
export type GetStateVersionResponse = z.infer<typeof getStateVersionResponseSchema>;
