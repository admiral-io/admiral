import { z } from 'zod';

const revisionSummarySchema = z.object({
  total: z.number().nullish(),
  succeeded: z.number().nullish(),
  failed: z.number().nullish(),
  blocked: z.number().nullish(),
  running: z.number().nullish(),
  cancelled: z.number().nullish(),
  pending: z.number().nullish(),
});

export const runSchema = z.object({
  id: z.string(),
  application_id: z.string().nullish(),
  environment_id: z.string().nullish(),
  status: z.string().nullish(),
  trigger_type: z.string().nullish(),
  triggered_by: z.string().nullish(),
  message: z.string().nullish(),
  destroy: z.boolean().nullish(),
  source_run_id: z.string().nullish(),
  change_set_id: z.string().nullish(),
  revision_summary: revisionSummarySchema.nullish(),
  created_at: z.string().nullish(),
  completed_at: z.string().nullish(),
});

export const listRunsSchema = z.object({
  runs: z.array(runSchema).default([]),
  next_page_token: z.string().nullish(),
});

const planSummarySchema = z.object({
  additions: z.number().nullish(),
  changes: z.number().nullish(),
  destructions: z.number().nullish(),
});

export const revisionSchema = z.object({
  id: z.string(),
  run_id: z.string().nullish(),
  component_id: z.string().nullish(),
  component_slug: z.string().nullish(),
  kind: z.string().nullish(),
  status: z.string().nullish(),
  source_id: z.string().nullish(),
  module_id: z.string().nullish(),
  version: z.string().nullish(),
  resolved_values: z.string().nullish(),
  depends_on: z.array(z.string()).nullish(),
  blocked_by: z.array(z.string()).nullish(),
  artifact_checksum: z.string().nullish(),
  artifact_url: z.string().nullish(),
  plan_summary: planSummarySchema.nullish(),
  has_plan_output: z.boolean().nullish(),
  error_message: z.string().nullish(),
  retry_count: z.number().nullish(),
  working_directory: z.string().nullish(),
  created_at: z.string().nullish(),
  started_at: z.string().nullish(),
  completed_at: z.string().nullish(),
});

export const listRevisionsSchema = z.object({
  revisions: z.array(revisionSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type Run = z.infer<typeof runSchema>;
export type RevisionSummary = z.infer<typeof revisionSummarySchema>;
export type Revision = z.infer<typeof revisionSchema>;
export type TerraformPlanSummary = z.infer<typeof planSummarySchema>;
export type ListRunsResponse = z.infer<typeof listRunsSchema>;
export type ListRevisionsResponse = z.infer<typeof listRevisionsSchema>;