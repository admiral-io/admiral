import { createListenerMiddleware } from '@reduxjs/toolkit';

import { type RootState } from '@/store/reducer';
import { setThemeMode } from '@/store/slices/user';

export const listenerMiddleware = createListenerMiddleware();

listenerMiddleware.startListening({
  actionCreator: setThemeMode,
  effect: (_, listenerApi) => {
    const prev = (listenerApi.getOriginalState() as RootState).user.preferences;
    const next = (listenerApi.getState() as RootState).user.preferences;
    if (prev.themeMode !== next.themeMode) {
      localStorage.setItem('user.preferences', JSON.stringify(next));
    }
  },
});
