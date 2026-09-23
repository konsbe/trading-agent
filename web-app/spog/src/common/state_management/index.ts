
/**
 * Central State Management Export
 * This file exports all state management utilities for sharing with micro frontends
 */

import { getGlobalStore, debugGlobalStore } from './utils/globalStoreUtils';

// Export the SAME global store instance
export const store = getGlobalStore();
export type { RootState, AppDispatch } from './store/dataStore';

// Export user slice and types
export { 
  defaultTheme,
  userDataSlice, 
  initUserDataStore, 
  clearUserDataStore, 
  updateUserDataStoreToken 
} from './slices/userData/userDataSlice';
export type { UserData, UserState } from './slices/userData/userDataSlice';

// Export hooks for easy consumption
export { 
  useUserData, 
  useUserToken, 
  useUserName, 
  useIsAuthenticated,
  useUserRoles 
} from './hooks/userData/useUserData';

// Export selectors for advanced usage
export * from './selectors/userData/userSelectors';

// Add debug export to verify store instance
export const debugStoreInstance = debugGlobalStore;
