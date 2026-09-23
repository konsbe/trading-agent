import * as React from 'react';
import { loadRemoteEntry } from './loadRemoteEntry';
import {
  deleteCachedModulePromise,
  getCachedModulePromise,
  setCachedModulePromise,
} from './mfeCache';

/**
 * Dynamically loads a Module Federation component from a remote container
 * @param scope - The global scope name for the remote container (e.g., 'mfe_topology_tree')
 * @param module - The module path to load (e.g., './TopologyTree')
 * @returns Promise that resolves to a React component
 */
export async function loadMfeComponent(
  scope: string,
  module: string
): Promise<React.ComponentType<any>> {
  const mfeCfg = (window as any).__APP_CONFIG__?.mfes?.[scope];
  const endpoint = mfeCfg?.endpoint;
  const resolvedScope = mfeCfg?.scope ?? scope;

  if (!endpoint) {
    throw new Error(
      `Unable to load remote entry for ${scope}. Missing endpoint in APPCONFIG (window.__APP_CONFIG__.mfes.${scope}.endpoint).`
    );
  }

  const cacheKey = `${resolvedScope}::${module}`;
  const cached = getCachedModulePromise<React.ComponentType<any>>(cacheKey);
  if (cached) return cached;

  const loadPromise = (async (): Promise<React.ComponentType<any>> => {
    // Initialize shared scope BEFORE loading the remote entry script.
    // The MFE's container-entry runs synchronously during script execution,
    // so __webpack_share_scopes__.default must exist before the script tag fires.
    await __webpack_init_sharing__('default');

    await loadRemoteEntry(endpoint, resolvedScope);

    // Poll until window[resolvedScope] is a valid MF container (must have .get).
    // The remoteEntry script may set window[scope] to the webpack chunk-push
    // array first, then replace it with the real {init, get} container object
    // once webpack finishes bootstrapping — so checking for truthiness alone
    // is not enough; we must check for the actual API surface.
    const pollIntervalMs = 100;
    const timeoutMs = 5000;
    const maxAttempts = Math.max(1, Math.ceil(timeoutMs / pollIntervalMs));
    let attempts = 0;

    while (attempts < maxAttempts) {
      const candidate = (window as any)[resolvedScope];
      if (candidate && typeof candidate.get === 'function') break;
      if (attempts === 0 && candidate) {
        // Something is at window[scope] but it's not an MF container — log it
        // so the developer can identify the mismatch (wrong MF name, etc.)
      }
      await new Promise(resolve => setTimeout(resolve, pollIntervalMs));
      attempts++;
    }

    const container = (window as any)[resolvedScope];
    if (!container || typeof container.get !== 'function') {
      const actualType = Array.isArray(container) ? 'Array' : typeof container;
      throw new Error(
        `Remote container "${resolvedScope}" is not a valid Module Federation container. ` +
        `window["${resolvedScope}"] has type ${actualType}. ` +
        `Ensure the MFE's webpack ModuleFederationPlugin name is exactly "${resolvedScope}".`
      );
    }

    // Initialize the container. Some self-initializing (MF v2 / SIC) containers
    // do not expose an .init method — skip gracefully in that case.
    if (typeof container.init === 'function') {
      await container.init(__webpack_share_scopes__.default);
    }

    // Re-initialize the host's shared scope after container.init.
    // Each remote container's init can corrupt entries in __webpack_share_scopes__.default
    // by setting version keys to null/undefined (explicitly enumerable but falsy).
    // Re-calling __webpack_init_sharing__ restores any host-known entries (via || pattern),
    // and then we delete any remaining null/undefined version entries. Without this,
    // later MFEs' consumes handlers find these entries via for..in, attempt
    // `entry.loaded = 1`, and throw TypeError: Cannot set properties of undefined.
    await __webpack_init_sharing__('default');
    const defaultScope = (__webpack_share_scopes__ as any).default as Record<string, Record<string, unknown>>;
    for (const moduleName in defaultScope) {
      const versions = defaultScope[moduleName];
      if (versions && typeof versions === 'object') {
        for (const version in versions) {
          if (versions[version] == null) {
            delete versions[version];
          }
        }
      }
    }

    // Get the module factory
    const factory = await container.get(module);
    const Module = factory();


    // Return the default export (should be a React component)
    if (Module && Module.default) {
      return Module.default as React.ComponentType<any>;
    }

    throw new Error(`Invalid MFE component from ${resolvedScope} module ${module}. No default export found.`);
  })().catch((err) => {
    // Don’t cache failures; allow retry on next navigation.
    deleteCachedModulePromise(cacheKey);
    throw err;
  });

  setCachedModulePromise(cacheKey, loadPromise);
  return loadPromise;
}
