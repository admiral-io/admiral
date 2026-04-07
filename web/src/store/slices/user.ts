import { createSelector, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { z } from 'zod';

import { type RootState } from '@/store/reducer';
import type { User } from '@/types/user';

export type ThemeMode = 'light' | 'dark' | 'system';

export interface UserPreferences {
  themeMode: ThemeMode;
}

const userPreferencesSchema = z.object({
  themeMode: z.enum(['light', 'dark', 'system']),
});

const defaultPreferences: UserPreferences = { themeMode: 'system' };

function loadPreferences(): UserPreferences {
  try {
    const raw = localStorage.getItem('user.preferences');
    if (!raw) return defaultPreferences;
    const result = userPreferencesSchema.safeParse(JSON.parse(raw));
    return result.success ? result.data : defaultPreferences;
  } catch {
    return defaultPreferences;
  }
}

export interface UserState {
  id: string;
  email: string;
  email_verified: boolean;
  display_name: string;
  given_name: string;
  family_name: string;
  avatar_url: string | undefined;
  preferences: UserPreferences;
}

const initialState: UserState = {
  id: '',
  email: '',
  email_verified: false,
  display_name: '',
  given_name: '',
  family_name: '',
  avatar_url: undefined,
  preferences: loadPreferences(),
};

const userSlice = createSlice({
  name: 'user',
  initialState,
  reducers: {
    setUser(state, action: PayloadAction<User>) {
      const u = action.payload;
      state.id = u.id;
      state.email = u.email;
      state.email_verified = u.email_verified ?? false;
      state.display_name = u.display_name ?? '';
      state.given_name = u.given_name ?? '';
      state.family_name = u.family_name ?? '';
      state.avatar_url = u.avatar_url;
    },
    setThemeMode(state, action: PayloadAction<ThemeMode>) {
      state.preferences.themeMode = action.payload ?? initialState.preferences.themeMode;
    },
  },
});

export const { setUser, setThemeMode } = userSlice.actions;

export const selectUser = (state: RootState): UserState => state.user;
export const selectUserProfile = createSelector(
  (state: RootState) => state.user,
  ({ id, email, email_verified, display_name, given_name, family_name, avatar_url }): User => ({
    id,
    email,
    email_verified,
    display_name,
    given_name,
    family_name,
    avatar_url,
  }),
);
export const selectUserPreferences = (state: RootState): UserPreferences => state.user.preferences;
export const selectThemeMode = (state: RootState): ThemeMode => state.user.preferences.themeMode;

export default userSlice.reducer;
