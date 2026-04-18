import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

export const applicationSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  description: z.string().nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listApplicationsSchema = z.object({
  applications: z.array(applicationSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type Application = z.infer<typeof applicationSchema>;
export type ListApplicationsResponse = z.infer<typeof listApplicationsSchema>;
