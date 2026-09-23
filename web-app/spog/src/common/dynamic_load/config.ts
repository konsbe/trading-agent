
/**
 * Loads application configuration from config.json
 * This replaces the need for build-time configuration
 */
export async function loadAppConfig(path: string = '/config.json'): Promise<AppConfig> {
  try {
    const res = await fetch(path, { cache: "no-store" });
    if (!res.ok) {
      throw new Error(`Failed to load config.json: ${res.status} ${res.statusText}`);
    }
    const config = (await res.json()) as AppConfig;
    return config;
  } catch (error) {
    // Failed to load application config
    throw error;
  }
}

/**
 * Get MFE enabled status from stored config
 */
export const isMfeEnabled = (mfeKey: string): boolean => {
  const config = window.__APP_CONFIG__;
  if (!config || !config.mfes || !config.mfes[mfeKey]) {
    return false;
  }
  const enabled = Boolean(config.mfes[mfeKey].enabled);
  return enabled;
};

// These will be evaluated when the functions are called (after config is loaded)
export const getMFE = (mfeKey: string): MFEConfigEntryWithRoles => {
  const config = window.__APP_CONFIG__;
  if(!config || !config.mfes || !config.mfes[mfeKey]) {
      return { label: '', version: '', endpoint: '', module: '', enabled: false, roles: [] };
  }
  const mfe = config.mfes[mfeKey];
  return { ...mfe, roles: mfe.roles ?? [] };
};

/** Iframe row from operator-generated `iframes` (legacy bundles omit this section). */
export const getIframeEntry = (iframeKey: string): MFEConfigEntryWithRoles => {
  const config = window.__APP_CONFIG__;
  if (!config?.iframes?.[iframeKey]) {
    return { label: '', version: '', endpoint: '', module: '', enabled: false, roles: [] };
  }
  const row = config.iframes[iframeKey];
  return { ...row, roles: row.roles ?? [] };
};

/**
 * Returns true only when the iframe entry exists in config.json and has enabled=true.
 * Consistent with isMfeEnabled: a missing entry means the feature is hidden.
 * An explicit enabled=true in config.json is required for the iframe to be shown.
 */
export const isIframeEnabled = (iframeKey: string): boolean => {
  const config = window.__APP_CONFIG__;
  if (!config?.iframes || !(iframeKey in config.iframes)) {
    return false;
  }
  return Boolean(config.iframes[iframeKey].enabled);
};

