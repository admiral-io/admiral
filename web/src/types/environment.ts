import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().optional(),
  display_name: z.string().optional(),
  email: z.string().optional(),
});

const kubernetesConfigSchema = z.object({
  cluster_id: z.string(),
  namespace: z.string().optional(),
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
  description: z.string().optional(),
  workload_targets: z.array(workloadTargetSchema).optional(),
  infrastructure_targets: z.array(infrastructureTargetSchema).optional(),
  labels: z.record(z.string(), z.string()).optional(),
  has_pending_changes: z.boolean().optional(),
  last_deployed_at: z.string().optional(),
  created_by: actorRefSchema.optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export const listEnvironmentsSchema = z.object({
  environments: z.array(environmentSchema).default([]),
  next_page_token: z.string().optional(),
});

export type WorkloadTarget = z.infer<typeof workloadTargetSchema>;
export type InfrastructureTarget = z.infer<typeof infrastructureTargetSchema>;
export type Environment = z.infer<typeof environmentSchema>;
export type ListEnvironmentsResponse = z.infer<typeof listEnvironmentsSchema>;
