import { z } from 'zod';

export const userSchema = z.object({
  id: z.uuid({ message: 'ID must be a valid UUID' }),
  email: z.string().email(),
  email_verified: z.boolean().default(false).optional(),
  display_name: z
    .string()
    .min(1, { message: 'Name must be at least 1 character' })
    .max(255, { message: 'Name must be 255 characters or less' })
    .nullish(),
  given_name: z.string().nullish(),
  family_name: z.string().nullish(),
  avatar_url: z
    .string()
    .nullish()
    .transform((val) => (val && val.trim() !== '' ? val : undefined))
    .pipe(z.string().url().optional()),
  created_at: z.iso
    .datetime({ message: 'Must be a valid date and time (e.g., 2024-03-15T10:30:00Z)' })
    .optional(),
  updated_at: z.iso
    .datetime({ message: 'Must be a valid date and time (e.g., 2024-03-15T10:30:00Z)' })
    .optional(),
});

export const userArraySchema = z.array(userSchema);

export const listUsersSchema = z.object({
  users: userArraySchema,
  next_page_token: z.string().optional(),
});

export type User = z.infer<typeof userSchema>;
export type UserList = z.infer<typeof userArraySchema>;
export type ListUsersResponse = z.infer<typeof listUsersSchema>;
