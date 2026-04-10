import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().optional(),
  display_name: z.string().optional(),
  email: z.string().optional(),
});

export const applicationSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  description: z.string().optional(),
  labels: z.record(z.string(), z.string()).optional(),
  created_by: actorRefSchema.optional(),
  updated_by: actorRefSchema.optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export const listApplicationsSchema = z.object({
  applications: z.array(applicationSchema).default([]),
  next_page_token: z.string().optional(),
});

export type Application = z.infer<typeof applicationSchema>;
export type ListApplicationsResponse = z.infer<typeof listApplicationsSchema>;
