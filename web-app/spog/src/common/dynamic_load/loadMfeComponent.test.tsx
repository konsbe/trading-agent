import React from 'react';
import { render, screen } from '@testing-library/react';
import { loadMfeComponent } from './loadMfeComponent';
import * as remoteEntryModule from './loadRemoteEntry';
import { clearAllMfeCaches } from './mfeCache';

declare const __webpack_init_sharing__: (scope: string) => Promise<void>;

describe('dynamic_load/loadMfeComponent', () => {
  beforeEach(() => {
    jest.useFakeTimers();

    jest.spyOn(remoteEntryModule, 'loadRemoteEntry').mockResolvedValue(undefined);

    // Minimal Module Federation globals needed by the loader
    (globalThis as any).__webpack_init_sharing__ = jest.fn(async () => undefined);
    (globalThis as any).__webpack_share_scopes__ = { default: {} };

    (window as any).__APP_CONFIG__ = {
      mfes: {
        mfeTest: {
          label: 'Test',
          version: '0.0.0',
          endpoint: 'http://example/remoteEntry.js',
          module: './Remote',
          enabled: true,
        },
      },
    };
  });

  afterEach(() => {
    jest.useRealTimers();
    jest.restoreAllMocks();
    clearAllMfeCaches();
    delete (window as any).mfeTest;
    delete (window as any).__APP_CONFIG__;
  });

  it('returns the default export from a remote module', async () => {
    const RemoteComponent = () => <div>remote</div>;

    (window as any).mfeTest = {
      init: jest.fn(async () => undefined),
      get: jest.fn(async () => () => ({ default: RemoteComponent })),
    };

    const Component = await loadMfeComponent('mfeTest', './Remote');
    render(<Component />);
    expect(screen.getByText('remote')).toBeInTheDocument();

    expect((globalThis as any).__webpack_init_sharing__).toHaveBeenCalledWith('default');
    expect((window as any).mfeTest.init).toHaveBeenCalled();
    expect((window as any).mfeTest.get).toHaveBeenCalledWith('./Remote');

    expect(remoteEntryModule.loadRemoteEntry).toHaveBeenCalledWith('http://example/remoteEntry.js', 'mfeTest');
  });

  it('loads remoteEntry from APPCONFIG', async () => {
    const RemoteComponent = () => <div>remote</div>;

    (window as any).mfeTest = {
      init: jest.fn(async () => undefined),
      get: jest.fn(async () => () => ({ default: RemoteComponent })),
    };

    const Component = await loadMfeComponent('mfeTest', './Remote');
    render(<Component />);
    expect(screen.getByText('remote')).toBeInTheDocument();

    expect(remoteEntryModule.loadRemoteEntry).toHaveBeenCalledWith('http://example/remoteEntry.js', 'mfeTest');
  });

  it('throws a helpful error when APPCONFIG has no endpoint', async () => {
    (window as any).__APP_CONFIG__ = { mfes: { mfeTest: { enabled: true } } };

    await expect(loadMfeComponent('mfeTest', './Remote')).rejects.toThrow(
      'Missing endpoint in APPCONFIG'
    );
  });

  it('throws a helpful error when the container never appears', async () => {
    // Attach the rejection handler immediately to avoid unhandled rejections
    // while we advance the fake timers.
    const expectation = expect(loadMfeComponent('mfeTest', './Remote')).rejects.toThrow(
      'Remote container "mfeTest" is not a valid Module Federation container'
    );

    // Advance enough time for the internal wait loop (50 * 100ms)
    await jest.advanceTimersByTimeAsync(5000);

    await expectation;
    expect((globalThis as any).__webpack_init_sharing__).toHaveBeenCalledWith('default');
  });

  it('throws when the loaded module has no default export', async () => {
    (window as any).mfeTest = {
      init: jest.fn(async () => undefined),
      get: jest.fn(async () => () => ({ notDefault: true })),
    };

    await expect(loadMfeComponent('mfeTest', './Remote')).rejects.toThrow(
      'Invalid MFE component from mfeTest module ./Remote. No default export found.'
    );
  });
});
