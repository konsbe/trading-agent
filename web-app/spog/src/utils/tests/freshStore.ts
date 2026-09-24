import { createStoreInstance } from '../../common/state_management/store/dataStore';

/**
 * Replaces the global store with a new one, as a page load would. Seed
 * localStorage first: the store reads the saved theme when it is created.
 */
export const installFreshStore = () => {
    const store = createStoreInstance();
    window.__TRADING_AGENT_REDUX_STORE__ = store;
    return store;
};
