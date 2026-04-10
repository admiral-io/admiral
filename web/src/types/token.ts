import { z } from 'zod';

const actorRefSchema = z.object({
  id: z.string().optional(),
  display_name: z.string().optional(),
  email: z.string().optional(),
});

export const accessTokenSchema = z.object({
  id: z.string(),
  name: z.string(),
  token_type: z.string().optional(),
  scopes: z.array(z.string()).default([]),
  status: z.string().optional(),
  binding_type: z.string().optional(),
  binding_id: z.string().optional(),
  created_by: actorRefSchema.nullish(),
  expires_at: z.string().nullish(),
  last_used_at: z.string().nullish(),
  created_at: z.string().nullish(),
  revoked_at: z.string().nullish(),
});

export const listTokensSchema = z.object({
  tokens: z.array(accessTokenSchema),
  next_page_token: z.string().optional(),
});

export const createTokenResponseSchema = z.object({
  access_token: accessTokenSchema,
  plain_text_token: z.string(),
});

export type AccessToken = z.infer<typeof accessTokenSchema>;
export type ListTokensResponse = z.infer<typeof listTokensSchema>;
export type CreateTokenResponse = z.infer<typeof createTokenResponseSchema>;
