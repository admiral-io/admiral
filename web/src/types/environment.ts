import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

const kubernetesConfigSchema = z.object({
  cluster_id: z.string(),
  namespace: z.string().nullish(),
});

const terraformConfigSchema = z.object({
  runner_id: z.string(),
});

const workloadTargetSchema = z.looseObject({
  kubernetes: kubernetesConfigSchema,
});

const infrastructureTargetSchema = z.looseObject({
  terraform: terraformConfigSchema,
});

export const environmentSchema = z.looseObject({
  id: z.uuid(),
  application_id: z.uuid(),
  name: z.string(),
  description: z.string().nullish(),
  workload_targets: z.array(workloadTargetSchema).nullish(),
  infrastructure_targets: z.array(infrastructureTargetSchema).nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  has_pending_changes: z.boolean().nullish(),
  last_deployed_at: z.string().nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listEnvironmentsSchema = z.object({
  environments: z.array(environmentSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type WorkloadTarget = z.infer<typeof workloadTargetSchema>;
export type InfrastructureTarget = z.infer<typeof infrastructureTargetSchema>;
export type Environment = z.infer<typeof environmentSchema>;
export type ListEnvironmentsResponse = z.infer<typeof listEnvironmentsSchema>;
