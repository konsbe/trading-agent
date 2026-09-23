import React, { useEffect } from 'react';
import { render, screen, waitFor } from '@testing-library/react';

const useMfeOperatorConfigSseMock = jest.fn();

jest.mock('../../common/sse/hooks/useMfeOperatorConfigSse', () => ({
  useMfeOperatorConfigSse: () => useMfeOperatorConfigSseMock(),
}));

const clearMfeCachesForScopeMock = jest.fn();

jest.mock('../../common/dynamic_load', () => ({
  clearMfeCachesForScope: (...args: any[]) => clearMfeCachesForScopeMock(...args),
}));

import SseMetadataProvider, { useMfeActivity, useMfeReloadToken } from './SseMetadataProvider';

const TokenViewer = ({ mfeKey, testId }: { mfeKey?: string; testId: string }) => {
  const token = useMfeReloadToken(mfeKey);
  return <div data-testid={testId}>{String(token)}</div>;
};

const ActiveMfe = ({ mfeKey }: { mfeKey: string }) => {
  const { registerActiveMfe, unregisterActiveMfe } = useMfeActivity();

  useEffect(() => {
    registerActiveMfe(mfeKey);
    return () => unregisterActiveMfe(mfeKey);
  }, [mfeKey, registerActiveMfe, unregisterActiveMfe]);

  return <div data-testid={`active-${mfeKey}`} />;
};

describe('SseMetadataProvider', () => {
  beforeEach(() => {
    useMfeOperatorConfigSseMock.mockReset();
    clearMfeCachesForScopeMock.mockReset();
    (global as any).fetch = jest.fn();
    (window as any).__APP_CONFIG__ = undefined;
    jest.spyOn(console, 'log').mockImplementation(() => undefined);
  });

  afterEach(() => {
      jest.restoreAllMocks();
      delete (window as any).__APP_CONFIG__;
      delete (global as any).fetch;
  });

  it('does nothing when there is no event', async () => {
    useMfeOperatorConfigSseMock.mockReturnValue({ event: null });

    render(
      <SseMetadataProvider>
        <div data-testid="child" />
      </SseMetadataProvider>
    );

    expect(global.fetch).not.toHaveBeenCalled();
    expect(clearMfeCachesForScopeMock).not.toHaveBeenCalled();
  });

  it('fetches config.json and updates APPCONFIG when an event is received', async () => {
    useMfeOperatorConfigSseMock.mockReturnValue({
      event: { event_type: 'mfeMetadataUpdate', event_origin: 'mfe-operator', timestamp: '1' },
    });

    const config = { shell_spog: { config: {} } };

    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => config,
    });

    render(
      <SseMetadataProvider>
        <div data-testid="child" />
      </SseMetadataProvider>
    );

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/config.json', { cache: 'no-store' });
    });

    await waitFor(() => {
      expect((window as any).__APP_CONFIG__).toEqual(config);
    });

  expect(clearMfeCachesForScopeMock).not.toHaveBeenCalled();
  });

  it('reloads an updated inactive MFE immediately (clears caches and bumps per-MFE token)', async () => {
    let currentEvent: any = null;
    useMfeOperatorConfigSseMock.mockImplementation(() => ({ event: currentEvent }));

    (window as any).__APP_CONFIG__ = {
      mfes: {
        mfeA: { version: '1.0.0', scope: 'scopeA' },
      },
    };

    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({
        mfes: {
          mfeA: { version: '1.0.1', scope: 'scopeA' },
        },
      }),
    });

    const { rerender } = render(
      <SseMetadataProvider>
        <TokenViewer mfeKey="mfeA" testId="token-mfeA" />
        <TokenViewer testId="token-global" />
      </SseMetadataProvider>
    );

    expect(screen.getByTestId('token-mfeA')).toHaveTextContent('0');
    expect(screen.getByTestId('token-global')).toHaveTextContent('0');

    currentEvent = { event_type: 'mfeMetadataUpdate', event_origin: 'mfe-operator', timestamp: '2' };
    rerender(
      <SseMetadataProvider>
        <TokenViewer mfeKey="mfeA" testId="token-mfeA" />
        <TokenViewer testId="token-global" />
      </SseMetadataProvider>
    );

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/config.json', { cache: 'no-store' });
    });

    await waitFor(() => {
      expect(clearMfeCachesForScopeMock).toHaveBeenCalledWith('scopeA');
    });

    await waitFor(() => {
      expect(screen.getByTestId('token-mfeA')).toHaveTextContent('1');
      expect(screen.getByTestId('token-global')).toHaveTextContent('1');
    });
  });

  it('defers reloading an updated active MFE until it becomes inactive', async () => {
    let currentEvent: any = null;
    useMfeOperatorConfigSseMock.mockImplementation(() => ({ event: currentEvent }));

    (window as any).__APP_CONFIG__ = {
      mfes: {
        mfeA: { version: '1.0.0', scope: 'scopeA' },
      },
    };

    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({
        mfes: {
          mfeA: { version: '1.0.1', scope: 'scopeA' },
        },
      }),
    });

    const { rerender } = render(
      <SseMetadataProvider>
        <ActiveMfe mfeKey="mfeA" />
        <TokenViewer mfeKey="mfeA" testId="token-mfeA" />
        <TokenViewer testId="token-global" />
      </SseMetadataProvider>
    );

    currentEvent = { event_type: 'mfeMetadataUpdate', event_origin: 'mfe-operator', timestamp: '3' };
    rerender(
      <SseMetadataProvider>
        <ActiveMfe mfeKey="mfeA" />
        <TokenViewer mfeKey="mfeA" testId="token-mfeA" />
        <TokenViewer testId="token-global" />
      </SseMetadataProvider>
    );

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/config.json', { cache: 'no-store' });
    });

    // While active, do not clear caches and do not bump the per-MFE token.
    expect(clearMfeCachesForScopeMock).not.toHaveBeenCalled();
    expect(screen.getByTestId('token-mfeA')).toHaveTextContent('0');

    // Global token still bumps for config refresh.
    await waitFor(() => {
      expect(screen.getByTestId('token-global')).toHaveTextContent('1');
    });

    // Unmount the active MFE: this should trigger the deferred reload.
    rerender(
      <SseMetadataProvider>
        <TokenViewer mfeKey="mfeA" testId="token-mfeA" />
        <TokenViewer testId="token-global" />
      </SseMetadataProvider>
    );

    await waitFor(() => {
      expect(clearMfeCachesForScopeMock).toHaveBeenCalledWith('scopeA');
    });

    await waitFor(() => {
      expect(screen.getByTestId('token-mfeA')).toHaveTextContent('1');
    });
  });
});
