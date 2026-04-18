import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().nullish(),
  display_name: z.string().nullish(),
  email: z.string().nullish(),
});

const basicAuthSchema = z.object({
  username: z.string().optional(),
  password: z.string().optional(),
});

const bearerTokenAuthSchema = z.object({
  token: z.string().optional(),
});

const sshKeyAuthSchema = z.object({
  private_key: z.string().optional(),
  passphrase: z.string().optional(),
});

const authConfigSchema = z.object({
  basic_auth: basicAuthSchema.optional(),
  bearer_token: bearerTokenAuthSchema.optional(),
  ssh_key: sshKeyAuthSchema.optional(),
});

export const credentialSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().nullish(),
  type: z.string().nullish(),
  auth_config: authConfigSchema.nullish(),
  labels: z.record(z.string(), z.string()).nullish(),
  created_by: actorRefSchema.nullish(),
  created_at: z.string().nullish(),
  updated_at: z.string().nullish(),
});

export const listCredentialsSchema = z.object({
  credentials: z.array(credentialSchema).default([]),
  next_page_token: z.string().nullish(),
});

export type Credential = z.infer<typeof credentialSchema>;
export type AuthConfig = z.infer<typeof authConfigSchema>;
export type ListCredentialsResponse = z.infer<typeof listCredentialsSchema>;
