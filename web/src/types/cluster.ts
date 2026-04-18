import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

export const clusterSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  cluster_uid: z.string().nullish(),
  health_status: z.string().nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listClustersSchema = z.object({
  clusters: z.array(clusterSchema).default([]),
  next_page_token: z.string().nullish(),
});

export const createClusterResponseSchema = z.object({
  cluster: clusterSchema,
  plain_text_token: z.string(),
});

export const clusterStatusSchema = z.object({
  k8s_version: z.string().nullish(),
  node_count: z.number().nullish(),
  nodes_ready: z.number().nullish(),
  pod_capacity: z.number().nullish(),
  pod_count: z.number().nullish(),
  pods_running: z.number().nullish(),
  pods_pending: z.number().nullish(),
  pods_failed: z.number().nullish(),
  cpu_capacity_millicores: z.union([z.number(), z.string()]).nullish(),
  cpu_used_millicores: z.union([z.number(), z.string()]).nullish(),
  memory_capacity_bytes: z.union([z.number(), z.string()]).nullish(),
  memory_used_bytes: z.union([z.number(), z.string()]).nullish(),
  workloads_total: z.number().nullish(),
  workloads_healthy: z.number().nullish(),
  workloads_degraded: z.number().nullish(),
  workloads_error: z.number().nullish(),
});

export const getClusterStatusResponseSchema = z.object({
  health_status: z.string().nullish(),
  status: clusterStatusSchema.nullish(),
  reported_at: z.string().nullish(),
});

const containerStatusSchema = z.object({
  name: z.string().nullish(),
  image: z.string().nullish(),
  restart_count: z.number().nullish(),
  state: z.string().nullish(),
  ready: z.boolean().nullish(),
});

export const workloadSchema = z.object({
  id: z.string().nullish(),
  cluster_id: z.string().nullish(),
  namespace: z.string().nullish(),
  name: z.string().nullish(),
  kind: z.string().nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  health_status: z.string().nullish(),
  status_reason: z.string().nullish(),
  replicas_desired: z.number().nullish(),
  replicas_ready: z.number().nullish(),
  replicas_available: z.number().nullish(),
  cpu_requests_millicores: z.union([z.number(), z.string()]).nullish(),
  cpu_limits_millicores: z.union([z.number(), z.string()]).nullish(),
  cpu_used_millicores: z.union([z.number(), z.string()]).nullish(),
  memory_requests_bytes: z.union([z.number(), z.string()]).nullish(),
  memory_limits_bytes: z.union([z.number(), z.string()]).nullish(),
  memory_used_bytes: z.union([z.number(), z.string()]).nullish(),
  containers: z.array(containerStatusSchema).nullish(),
  last_updated_at: z.string().nullish(),
});

export const listWorkloadsSchema = z.object({
  workloads: z.array(workloadSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type Cluster = z.infer<typeof clusterSchema>;
export type ClusterStatus = z.infer<typeof clusterStatusSchema>;
export type GetClusterStatusResponse = z.infer<typeof getClusterStatusResponseSchema>;
export type CreateClusterResponse = z.infer<typeof createClusterResponseSchema>;
export type Workload = z.infer<typeof workloadSchema>;
export type ContainerStatus = z.infer<typeof containerStatusSchema>;
export type ListClustersResponse = z.infer<typeof listClustersSchema>;
export type ListWorkloadsResponse = z.infer<typeof listWorkloadsSchema>;
