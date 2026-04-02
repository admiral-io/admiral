import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import storage from 'redux-persist/es/storage';

import { type RootState } from '@/store/reducer';
import { type MenuState } from '@/types/menu';

export const persistConfig = {
  key: 'menu',
  keyPrefix: 'admiral:',
  storage,
};

const initialState: MenuState = {
  selectedItem: ['dashboard'],
  selectedID: null,
  drawerOpen: false,
  menu: {},
};

const menuSlice = createSlice({
  name: 'menu',
  initialState,
  reducers: {
    activeItem(state, action: PayloadAction<string[]>) {
      state.selectedItem = action.payload;
    },

    activeID(state, action: PayloadAction<string | null>) {
      state.selectedID = action.payload;
    },

    openDrawer(state, action: PayloadAction<boolean>) {
      state.drawerOpen = action.payload;
    },

    getMenuSuccess(state, action: PayloadAction<Record<string, unknown>>) {
      state.menu = action.payload;
    },
  },
});

export const { activeItem, activeID, openDrawer, getMenuSuccess } = menuSlice.actions;

export const selectMenu = (state: RootState): MenuState => state.menu;

export default menuSlice.reducer;