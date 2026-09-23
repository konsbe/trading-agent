import { getMFE, isMfeEnabled, loadAppConfig, getIframeEntry, isIframeEnabled } from './config';

describe('common/config', () => {
  const originalFetch = (globalThis as any).fetch;

  afterEach(() => {
    // cleanup global config between tests
    (window as any).__APP_CONFIG__ = undefined;

    (globalThis as any).fetch = originalFetch;
    jest.restoreAllMocks();
  });

  describe('loadAppConfig', () => {
    it('loads and returns JSON config when fetch is ok', async () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/remoteEntry.js',
            module: './A',
            enabled: true,
          },
        },
      };

      const fetchMock = ((globalThis as any).fetch = jest.fn().mockResolvedValue({
        ok: true,
        status: 200,
        statusText: 'OK',
        json: async () => config,
      } as any));

      const result = await loadAppConfig('/config.json');
      expect(fetchMock).toHaveBeenCalledWith('/config.json', { cache: 'no-store' });
      expect(result).toEqual(config);
    });

    it('throws a useful error when fetch returns non-ok', async () => {
      (globalThis as any).fetch = jest.fn().mockResolvedValue({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({}),
      } as any);

      await expect(loadAppConfig('/config.json')).rejects.toThrow(
        'Failed to load config.json: 404 Not Found'
      );
    });

    it('propagates fetch errors', async () => {
      (globalThis as any).fetch = jest.fn().mockRejectedValue(new Error('network down'));
      await expect(loadAppConfig('/config.json')).rejects.toThrow('network down');
    });
  });


  describe('isMfeEnabled', () => {
    it('checks enabled status from stored config', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: true,
          },
          mfeB: {
            label: 'B',
            version: '1.0.0',
            endpoint: 'http://example/b',
            module: './B',
            enabled: false,
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;
      expect(isMfeEnabled('mfeA')).toBe(true);
      expect(isMfeEnabled('mfeB')).toBe(false);
      expect(isMfeEnabled('doesNotExist')).toBe(false);
    });

    it('returns false when config is not stored', () => {
      expect(isMfeEnabled('mfeA')).toBe(false);
    });

    it('returns false when config.mfes is undefined', () => {
      (window as any).__APP_CONFIG__ = {};
      expect(isMfeEnabled('mfeA')).toBe(false);
    });

    it('handles string "true" as enabled', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: 'true',
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;
      expect(isMfeEnabled('mfeA')).toBe(true);
    });

    it('handles empty string as disabled', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: '',
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;
      expect(isMfeEnabled('mfeA')).toBe(false);
    });
  });


  describe('getMFE', () => {
    it('returns MFE config with roles when MFE exists', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: true,
            roles: ['admin', 'user'],
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;

      const mfe = getMFE('mfeA');
      expect(mfe).toEqual({
        label: 'A',
        version: '1.0.0',
        endpoint: 'http://example/a',
        module: './A',
        enabled: true,
        roles: ['admin', 'user'],
      });
    });

    it('returns MFE config with empty roles array when roles is undefined', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: true,
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;

      const mfe = getMFE('mfeA');
      expect(mfe).toEqual({
        label: 'A',
        version: '1.0.0',
        endpoint: 'http://example/a',
        module: './A',
        enabled: true,
        roles: [],
      });
    });

    it('returns default empty config when MFE does not exist', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: true,
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;

      const mfe = getMFE('doesNotExist');
      expect(mfe).toEqual({
        label: '',
        version: '',
        endpoint: '',
        module: '',
        enabled: false,
        roles: [],
      });
    });

    it('returns default empty config when config is not loaded', () => {
      const mfe = getMFE('mfeA');
      expect(mfe).toEqual({
        label: '',
        version: '',
        endpoint: '',
        module: '',
        enabled: false,
        roles: [],
      });
    });

    it('returns default empty config when config.mfes is undefined', () => {
      (window as any).__APP_CONFIG__ = {};

      const mfe = getMFE('mfeA');
      expect(mfe).toEqual({
        label: '',
        version: '',
        endpoint: '',
        module: '',
        enabled: false,
        roles: [],
      });
    });

    it('includes all optional fields when present', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: true,
            roles: ['admin'],
            description: 'Test MFE',
            isConfigMfe: true,
            scope: 'customScope',
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;

      const mfe = getMFE('mfeA');
      expect(mfe).toEqual({
        label: 'A',
        version: '1.0.0',
        endpoint: 'http://example/a',
        module: './A',
        enabled: true,
        roles: ['admin'],
        description: 'Test MFE',
        isConfigMfe: true,
        scope: 'customScope',
      });
    });

    it('handles enabled as string value', () => {
      const config: AppConfig = {
        mfes: {
          mfeA: {
            label: 'A',
            version: '1.0.0',
            endpoint: 'http://example/a',
            module: './A',
            enabled: 'true',
            roles: ['admin'],
          },
        },
      };

      (window as any).__APP_CONFIG__ = config;

      const mfe = getMFE('mfeA');
      expect(mfe.enabled).toBe('true');
    });
  });

  describe('iframe helpers', () => {
    it('getIframeEntry returns defaults when missing', () => {
      (window as any).__APP_CONFIG__ = { mfes: {} };
      expect(getIframeEntry('iframe_homer_ui').endpoint).toBe('');
    });

    it('getIframeEntry reads iframes section', () => {
      (window as any).__APP_CONFIG__ = {
        mfes: {},
        iframes: {
          'iframe_homer_ui': {
            label: 'H',
            version: '1',
            endpoint: 'https://homer.example/',
            module: './HomerUI',
            enabled: true,
            roles: ['r1'],
          },
        },
      };
      const row = getIframeEntry('iframe_homer_ui');
      expect(row.endpoint).toBe('https://homer.example/');
      expect(row.roles).toEqual(['r1']);
    });

    it('isIframeEnabled is false when iframes omitted (no explicit entry = hidden)', () => {
      (window as any).__APP_CONFIG__ = { mfes: {} };
      expect(isIframeEnabled('iframe_homer_ui')).toBe(false);
    });

    it('isIframeEnabled is true when entry is enabled', () => {
      (window as any).__APP_CONFIG__ = {
        mfes: {},
        iframes: {
          'iframe_homer_ui': {
            label: 'H', version: '1', endpoint: 'https://x', module: './M', enabled: true,
          },
        },
      };
      expect(isIframeEnabled('iframe_homer_ui')).toBe(true);
    });

    it('isIframeEnabled is false when entry disabled', () => {
      (window as any).__APP_CONFIG__ = {
        mfes: {},
        iframes: {
          'iframe_homer_ui': {
            label: 'H',
            version: '1',
            endpoint: 'https://x',
            module: './M',
            enabled: false,
          },
        },
      };
      expect(isIframeEnabled('iframe_homer_ui')).toBe(false);
    });

  });
});
