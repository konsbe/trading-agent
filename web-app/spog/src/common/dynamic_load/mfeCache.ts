type ModulePromise = Promise<unknown>;

// Cache for `loadMfeComponent(scope,module)` calls.
// Important: if a load fails, we delete the key so callers can retry.
const modulePromiseCache = new Map<string, ModulePromise>();

export const getCachedModulePromise = <T = unknown>(key: string): Promise<T> | undefined => {
  return modulePromiseCache.get(key) as Promise<T> | undefined;
};

export const setCachedModulePromise = <T = unknown>(key: string, value: Promise<T>): void => {
  modulePromiseCache.set(key, value as ModulePromise);
};

export const deleteCachedModulePromise = (key: string): void => {
  modulePromiseCache.delete(key);
};

const ALLOWED_SCOPE_KEY_CHARS = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_';

const sanitizeScopeForGlobalKey = (scope: string): string => {
  const out = scope.split('').filter(ch => ALLOWED_SCOPE_KEY_CHARS.includes(ch)).join('');
  return out;
};

const clearWebpackChunkGlobalsForScope = (scope: string): void => {
  const sanitizedScope = sanitizeScopeForGlobalKey(scope);
  const chunkGlobals = new Set<string>([
    `webpackChunk${scope}`,
    `webpackChunk${sanitizedScope}`,
  ]);

  for (const key of chunkGlobals) {
    try {
      delete (window as any)[key];
    } catch {
      (window as any)[key] = undefined;
    }
  }
};

export const clearMfeCachesForScope = (scope: string): void => {
  // Clear cached module promises for this container scope.
  const prefix = `${scope}::`;

  Array.from(modulePromiseCache.keys())
    .filter(key => key.startsWith(prefix))
    .forEach(key => modulePromiseCache.delete(key));

  if (typeof document === 'undefined') return;

  // Remove the remoteEntry script tag and delete related globals.
  const scripts = Array.from(document.querySelectorAll<HTMLScriptElement>(`script[data-mfe-scope="${scope}"]`));
  for (const script of scripts) {
    try {
      delete (window as any)[scope];
    } catch {
      (window as any)[scope] = undefined;
    }

    clearWebpackChunkGlobalsForScope(scope);
    script.parentNode?.removeChild(script);
  }
};

/**
 * Clears client-side caches for dynamic MFEs:
 * - cached `loadMfeComponent` promises
 * - remoteEntry script tags inserted via `loadRemoteEntry` (identified by `data-mfe-scope`)
 * - associated `window[scope]` container globals
 */
export const clearAllMfeCaches = (): void => {
  modulePromiseCache.clear();

  if (typeof document === 'undefined') return;

  const scripts = Array.from(document.querySelectorAll<HTMLScriptElement>('script[data-mfe-scope]'));
  for (const script of scripts) {
    const scope = script.getAttribute('data-mfe-scope');
    if (scope) {
      try {
        delete (window as any)[scope];
      } catch {
        (window as any)[scope] = undefined;
      }

      // Webpack remotes typically create a global chunk loading array like:
      //   window.webpackChunk<uniqueName>
      // If we don't clear it, a newly loaded remoteEntry can replay old chunks and keep old module code alive.
      clearWebpackChunkGlobalsForScope(scope);
    }

    script.parentNode?.removeChild(script);
  }
};
