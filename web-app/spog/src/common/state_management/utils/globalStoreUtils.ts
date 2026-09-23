/**
 * Global Store Utilities
 * Ensures single Redux store instance across all webpack bundles and Module Federation
 */

// Declare global store type
declare global {
  interface Window {
    __TRADING_AGENT_REDUX_STORE__?: any;
  }
}

/**
 * Get the global store instance - ensures same store across all bundles
 * This is the ONLY place where we manage the global store logic
 */
export const getGlobalStore = () => {
  if (typeof window !== 'undefined' && window.__TRADING_AGENT_REDUX_STORE__) {
    return window.__TRADING_AGENT_REDUX_STORE__;
  }
  
  // Fallback to direct import and set global
  const store = require('../store/dataStore').default;
  
  if (typeof window !== 'undefined') {
    window.__TRADING_AGENT_REDUX_STORE__ = store;
    // SHELL: Stored globally for cross-bundle access
  }
  
  return store;
};

/**
 * Debug function to verify store instance
 */
export const debugGlobalStore = () => {
  const store = getGlobalStore();
  // SHELL: Global store instance store
  // SHELL: Current state: store.getState());
  return store;
};
