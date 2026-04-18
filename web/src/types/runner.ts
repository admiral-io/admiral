import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

export const runnerSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  kind: z.string().nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  health_status: z.string().nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listRunnersSchema = z.object({
  runners: z.array(runnerSchema).default([]),
  next_page_token: z.string().nullish(),
});

export const createRunnerResponseSchema = z.object({
  runner: runnerSchema,
  plain_text_token: z.string(),
});

const activeJobInfoSchema = z.object({
  job_id: z.string().nullish(),
  phase: z.string().nullish(),
  started_at: z.string().nullish(),
});

export const runnerStatusSchema = z.object({
  version: z.string().nullish(),
  active_jobs: z.number().nullish(),
  max_concurrent_jobs: z.number().nullish(),
  available_providers: z.array(z.string()).nullish(),
  tool_versions: z.record(z.string(), z.string()).nullish(),
  active_job_details: z.array(activeJobInfoSchema).nullish(),
});

export const getRunnerStatusResponseSchema = z.object({
  health_status: z.string().nullish(),
  status: runnerStatusSchema.nullish(),
  reported_at: z.string().nullish(),
});

export const jobSchema = z.object({
  id: z.string(),
  runner_id: z.string().nullish(),
  revision_id: z.string().nullish(),
  deployment_id: z.string().nullish(),
  job_type: z.string().nullish(),
  status: z.string().nullish(),
  created_at: z.string().nullish(),
  started_at: z.string().nullish(),
  completed_at: z.string().nullish(),
});

export const listRunnerJobsSchema = z.object({
  jobs: z.array(jobSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type Runner = z.infer<typeof runnerSchema>;
export type RunnerStatus = z.infer<typeof runnerStatusSchema>;
export type GetRunnerStatusResponse = z.infer<typeof getRunnerStatusResponseSchema>;
export type Job = z.infer<typeof jobSchema>;
export type CreateRunnerResponse = z.infer<typeof createRunnerResponseSchema>;
export type ListRunnersResponse = z.infer<typeof listRunnersSchema>;
export type ListRunnerJobsResponse = z.infer<typeof listRunnerJobsSchema>;
