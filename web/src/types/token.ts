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
  created_by: actorRefSchema.optional(),
  expires_at: z.string().optional(),
  last_used_at: z.string().optional(),
  created_at: z.string().optional(),
  revoked_at: z.string().optional(),
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
