import { createListenerMiddleware } from '@reduxjs/toolkit';

import { type RootState } from '@/store/reducer';
import { setThemeMode } from '@/store/slices/user';

export const listenerMiddleware = createListenerMiddleware();

listenerMiddleware.startListening({
  actionCreator: setThemeMode, // isAnyOf for multiples
  effect: (_, listenerApi) => {
    localStorage.setItem(
      'user.preferences',
      JSON.stringify((listenerApi.getState() as RootState).user.preferences),
    );
  },
});
