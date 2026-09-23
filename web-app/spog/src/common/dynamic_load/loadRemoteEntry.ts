/**
 * Dynamically loads a Module Federation remote entry script
 * @param remoteUrl - Full URL to the remoteEntry.js file
 * @param scope - The global scope name for the remote container (e.g., 'mfeTopology')
 * @returns Promise that resolves when the script is loaded
 */
export async function loadRemoteEntry(remoteUrl: string, scope: string): Promise<void> {
  return new Promise((resolve, reject) => {
    // If the container is already a valid MF container, nothing to do
    const existing = (window as any)[scope];
    if (existing && typeof existing.get === 'function') {
      resolve();
      return;
    }

    // Check if script is already in DOM (but may still be loading)
    const existingScript = document.querySelector(`script[data-mfe-scope="${scope}"]`);
    if (existingScript) {
      const containerCheck = (window as any)[scope];
      if (containerCheck && typeof containerCheck.get === 'function') {
        resolve();
        return;
      }
      // Script tag exists but container not yet registered — wait for it
      existingScript.addEventListener('load', () => {
        resolve(); // loadMfeComponent will poll for the real container
      });
      existingScript.addEventListener('error', () => {
        reject(new Error(`Failed to load remote ${scope}`));
      });
      return;
    }

    const script = document.createElement('script');
    script.src = remoteUrl;
    script.type = 'text/javascript';
    script.async = true;
    script.setAttribute('data-mfe-scope', scope);

    script.onload = () => {
      // Script loaded — let loadMfeComponent's polling validate the container.
      // We don't reject here even if window[scope] isn't a container yet:
      // async-initializing remotes (MF v2) set the container after the script runs.
      resolve();
    };

    script.onerror = (error) => {
      script.remove();
      reject(new Error(`Failed to load remote ${scope} from ${remoteUrl}`));
    };

    document.head.appendChild(script);
  });
}
