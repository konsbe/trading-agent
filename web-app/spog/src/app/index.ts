import { bootstrapApp } from './bootstrap';
import { loadAppConfig } from '../common/dynamic_load';

/**
 * Initialize application with dynamic MFE loading
 * 1. Load config.json and store globally in window.__APP_CONFIG__
 * 2. Set webpack public path for correct MFE chunk loading
 * 3. Set legacy RUNTIME_CONFIG for backward compatibility
 * 4. Bootstrap React application
 */
(async () => {
    try {
        // Step 1: Reuse config already set by the HTML inline loader, or fetch as fallback.
        // The index.html script fetches config.json and sets window.__APP_CONFIG__ *before*
        // bundle.js is appended to the DOM, so by the time this code runs the config is ready.
        const config = window.__APP_CONFIG__ ?? (await loadAppConfig());
        window.__APP_CONFIG__ = config;

        // Step 2: Set webpack public path for Module Federation chunk loading
        // Must happen before any lazy MFE chunks are requested
        /**
         * Dynamic webpack public path configuration for Module Federation
         * @see https://webpack.js.org/guides/public-path/#on-the-fly
         * The __webpack_public_path__ assignment must happen before any other imports because:
         * Webpack chunks are loaded during import resolution
         * 
         * Once an import starts, webpack uses the current publicPath
         * Webpack builds your shell and creates multiple files (remoteEntry.js, bundle.js, 257.bundle.js, 558.bundle.js...)
         * At build time, webpack doesn't know the final deployment path (e.g. /mfe-data/spog-1.100.294/)
         * With the dynamic public path, webpack loads chunks from the correct versioned path: https://domain.com/mfe-data/spog-1.100.294/257.bundle.js
         * How it Works: 
         * Shell loads → config.json contains MFE definitions
         * This code runs → Sets __webpack_public_path__ to / mfe - data / spog - 1.100.294 /
         * Webpack automatically uses this path → All subsequent chunk requests use the correct path
         * Module Federation works → Your MFE can access StateManagement because chunks load properly
        */
        const mainJsUrl = config.shell_spog?.mainJsUrl;
        if (mainJsUrl) {
            const absoluteUrl = mainJsUrl.startsWith('/')
                ? window.location.origin + mainJsUrl
                : mainJsUrl;
            __webpack_public_path__ = String(new URL('.', absoluteUrl));
        }

        // Step 3: Set legacy RUNTIME_CONFIG
        // TODO: remove once all MFEs are migrated to use __APP_CONFIG__
        window.RUNTIME_CONFIG = {
            SHELL: {
                SHELL_SPOG_REMOTE_URL:
                    (config.shell_spog?.config?.spogDomain ?? '') +
                    (config.shell_spog?.endpoint ?? ''),
            },
        };

        // Step 4: Set page title from config
        const bannerString = config.shell_spog?.config?.bannerString;
        if (bannerString) {
            document.title = bannerString;
        }

        // Step 5: Bootstrap the application
        bootstrapApp();

    } catch (error) {
        // Show user-friendly error
        document.body.innerHTML = `
            <div style="display: flex; align-items: center; justify-content: center; height: 100vh; font-family: sans-serif;">
                <div style="text-align: center;">
                    <h1>Application Initialization Failed</h1>
                    <p>Unable to load application configuration.</p>
                    <p style="color: #666;">${error instanceof Error ? error.message : 'Unknown error'}</p>
                    <button onclick="location.reload()" style="margin-top: 20px; padding: 10px 20px; cursor: pointer;">
                        Retry
                    </button>
                </div>
            </div>
        `;
    }
})();
