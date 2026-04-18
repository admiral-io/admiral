import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

export const variableSchema = z.object({
  id: z.string(),
  key: z.string(),
  value: z.string().nullish(),
  sensitive: z.boolean().nullish(),
  type: z.string().nullish(),
  description: z.string().nullish(),
  application_id: z.string().nullish(),
  environment_id: z.string().nullish(),
  source: z.string().nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listVariablesSchema = z.object({
  variables: z.array(variableSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type Variable = z.infer<typeof variableSchema>;
export type ListVariablesResponse = z.infer<typeof listVariablesSchema>;
