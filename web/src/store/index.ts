import {
  type Action,
  configureStore,
  type ThunkAction,
  type ConfigureStoreOptions,
} from '@reduxjs/toolkit';
import {
  type TypedUseSelectorHook,
  useDispatch as useReduxDispatch,
  useSelector as useReduxSelector,
} from 'react-redux';
import { FLUSH, REHYDRATE, PAUSE, PERSIST, PURGE, REGISTER } from 'redux-persist';
import { createLogger } from 'redux-logger';
import { persistStore } from 'redux-persist';

import { listenerMiddleware } from '@/store/middleware';
import rootReducer from '@/store/reducer';
import type { RootState } from '@/store/reducer';

export type { RootState } from '@/store/reducer';

const middleware: ConfigureStoreOptions['middleware'] = (getDefaultMiddleware) => {
  return getDefaultMiddleware({
    serializableCheck: {
      ignoredActions: [FLUSH, REHYDRATE, PAUSE, PERSIST, PURGE, REGISTER],
    },
  })
    .concat(listenerMiddleware.middleware)
    .concat(import.meta.env.DEV ? [createLogger()] : []);
};

const store = configureStore({
  reducer: rootReducer,
  middleware,
  devTools: import.meta.env.DEV,
});

const persistor = persistStore(store);

export type AppDispatch = typeof store.dispatch;

export type AppThunk<R = void> = ThunkAction<R, RootState, unknown, Action<string>>;

// Typed hooks
const useDispatch = (): AppDispatch => useReduxDispatch<AppDispatch>();
const useSelector: TypedUseSelectorHook<RootState> = useReduxSelector;

export { store, persistor, useDispatch, useSelector };
