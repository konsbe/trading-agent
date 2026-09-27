/**
 * Build-time helper (required from webpack.config.js via ts-node).
 *
 * Returns a Module Federation "promise" remote that resolves the spog shell
 * container at runtime from `window.__APP_CONFIG__.shell_spog` (loaded by spog
 * from config.json), so the production bundle carries no hard-coded shell URL.
 * Falls back to `<page origin>/remoteEntry.js`, which is where spog serves it
 * when this MFE is hosted inside the shell.
 */
export const SHELL_CONTAINER_NAME = 'shell_spog';

export function createShellRemotePromise(containerName: string = SHELL_CONTAINER_NAME): string {
  return `promise new Promise((resolve, reject) => {
    const containerName = ${JSON.stringify(containerName)};
    if (window[containerName] && typeof window[containerName].get === 'function') {
      resolve(window[containerName]);
      return;
    }

    const shellCfg = (window.__APP_CONFIG__ && window.__APP_CONFIG__.shell_spog) || {};
    const base = (shellCfg.config && shellCfg.config.spogDomain) || window.location.origin;
    const endpoint = shellCfg.endpoint || '/remoteEntry.js';
    let remoteUrl;
    try {
      remoteUrl = new URL(endpoint, base).toString();
    } catch (error) {
      reject(new Error('Invalid spog remote URL: ' + endpoint + ' (base ' + base + ')'));
      return;
    }

    const script = document.createElement('script');
    script.src = remoteUrl;
    script.type = 'text/javascript';
    script.async = true;
    script.onload = () => {
      const container = window[containerName];
      if (!container) {
        reject(new Error('spog container not found on window: ' + containerName));
        return;
      }
      resolve(container);
    };
    script.onerror = () => reject(new Error('Failed to load spog remote entry from ' + remoteUrl));
    document.head.appendChild(script);
  })`;
}
