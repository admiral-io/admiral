import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import { userSchema, type User } from '@/types/user';

export async function getMe(): Promise<User> {
  const { data, error } = await client.GET('/api/v1/user/me');

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to fetch user profile');
  }

  const u = data.user;
  if (!u) {
    throw new Error('No user returned from API');
  }

  return userSchema.parse({
    id: u.id ?? '',
    email: u.email ?? '',
    emailVerified: u.email_verified,
    name: u.display_name ?? undefined,
    givenName: u.given_name ?? undefined,
    familyName: u.family_name ?? undefined,
    pictureUrl: u.avatar_url ?? undefined,
    createdAt: u.created_at ?? undefined,
    updatedAt: u.updated_at ?? undefined,
  });
}
