import { configureStore } from '@reduxjs/toolkit';
import { userDataSlice } from '../slices/userData/userDataSlice';
import type { UserState } from '../slices/userData/userDataSlice';
import { filterDataSlice } from '../slices/filterData/filterDataSlice';

export interface RootState {
  user: UserState;
  filterData: ReturnType<typeof filterDataSlice.reducer>;
}

// Declare global store type
declare global {
  interface Window {
    __TRADING_AGENT_REDUX_STORE__?: any;
  }
}

// Create store instance ONCE globally
let globalStore: any = null;

const createStoreInstance = () => {
  return configureStore({
    reducer: {
      filterData: filterDataSlice.reducer,
      user: userDataSlice.reducer,
    },
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware({
        serializableCheck: {
          ignoredActions: ['persist/PERSIST'],
        },
      }),
    // Redux DevTools configuration
    devTools: process.env.NODE_ENV !== 'production' && {
      name: 'Trading Agent SPOG Store',
      trace: false,
      serialize: false,
    },
  });
};

// Ensure single store instance across all bundles
if (typeof window !== 'undefined' && window.__TRADING_AGENT_REDUX_STORE__) {
  globalStore = window.__TRADING_AGENT_REDUX_STORE__;
  // SHELL: Using existing global store instance: globalStore
} else if (!globalStore) {
  globalStore = createStoreInstance();
  // SHELL: Created new global store instance: globalStore
  // SHELL: Initial store state: globalStore.getState()
  
  // Store globally for cross-bundle access
  if (typeof window !== 'undefined') {
    window.__TRADING_AGENT_REDUX_STORE__ = globalStore;
  }
} else {
  // SHELL: Using existing module-level global store instance
}

const store = globalStore;

export type AppDispatch = typeof store.dispatch;
export default store;
