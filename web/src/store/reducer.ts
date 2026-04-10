import { combineReducers } from 'redux';
import { persistReducer } from 'redux-persist';

import menuReducer, { persistConfig as menuPersistConfig } from '@/store/slices/menu';
import userReducer from '@/store/slices/user';
import snackbarReducer from '@/store/slices/snackbar';

// Combine your reducers into the root reducer
const reducer = combineReducers({
  user: userReducer,
  menu: persistReducer(menuPersistConfig, menuReducer),
  snackbar: snackbarReducer,
});

// Derive RootState from the root reducer
export type RootState = ReturnType<typeof reducer>;

export default reducer;
