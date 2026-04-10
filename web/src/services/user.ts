import { client } from '@/services/client';
import { parseApiError } from '@/services/errors';
import { userSchema, type User } from '@/types/user';

export async function getMe(): Promise<User> {
  const { data, error } = await client.GET('/api/v1/user/me');

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to fetch user profile');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.user) {
    throw new Error('No user returned from API');
  }

  return userSchema.parse(raw.user);
}

export async function get(userId: string): Promise<User> {
  const { data, error } = await client.GET('/api/v1/users/{user_id}', {
    params: { path: { user_id: userId } },
  });

  if (error) {
    throw parseApiError(error) ?? new Error('Failed to fetch user');
  }

  const raw = data as Record<string, unknown>;
  if (!raw.user) {
    throw new Error('No user returned from API');
  }

  return userSchema.parse(raw.user);
}
