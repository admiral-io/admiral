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
  emailVerified: boolean;
  name: string;
  givenName: string;
  familyName: string;
  pictureUrl: string | undefined;
  preferences: UserPreferences;
}

const initialState: UserState = {
  id: '',
  email: '',
  emailVerified: false,
  name: '',
  givenName: '',
  familyName: '',
  pictureUrl: undefined,
  preferences: loadPreferences(),
};

const userSlice = createSlice({
  name: 'user',
  initialState,
  reducers: {
    setUser(state, action: PayloadAction<User>) {
      const { id, email, emailVerified, name, givenName, familyName, pictureUrl } = action.payload;
      state.id = id;
      state.email = email;
      state.emailVerified = emailVerified ?? false;
      state.name = name ?? '';
      state.givenName = givenName ?? '';
      state.familyName = familyName ?? '';
      state.pictureUrl = pictureUrl;
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
  ({ id, email, emailVerified, name, givenName, familyName, pictureUrl }): User => ({
    id,
    email,
    emailVerified,
    name,
    givenName,
    familyName,
    pictureUrl,
  }),
);
export const selectUserPreferences = (state: RootState): UserPreferences => state.user.preferences;
export const selectThemeMode = (state: RootState): ThemeMode => state.user.preferences.themeMode;

export default userSlice.reducer;
