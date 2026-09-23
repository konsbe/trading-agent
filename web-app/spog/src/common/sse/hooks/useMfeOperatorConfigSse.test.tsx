import React from 'react';
import { render, screen, act } from '@testing-library/react';

const subscribeMock = jest.fn();
const unsubscribeMock = jest.fn();

jest.mock('../services/SseService', () => ({
  sseService: {
    subscribe: (...args: any[]) => subscribeMock(...args),
    unsubscribe: (...args: any[]) => unsubscribeMock(...args),
  },
}));

import { useMfeOperatorConfigSse } from './useMfeOperatorConfigSse';

function TestComponent() {
  const { event } = useMfeOperatorConfigSse();
  return (
    <div>
      <div data-testid="event_type">{event?.event_type ?? ''}</div>
      <div data-testid="origin">{event?.event_origin ?? ''}</div>
    </div>
  );
}

describe('useMfeOperatorConfigSse', () => {
  let consoleLogSpy: jest.SpyInstance;

  beforeEach(() => {
    subscribeMock.mockReset();
    unsubscribeMock.mockReset();

    consoleLogSpy = jest.spyOn(console, 'log').mockImplementation(() => undefined);
  });

  afterEach(() => {
    consoleLogSpy.mockRestore();
  });

  it('subscribes to the same-origin /mfe_operator_events endpoint', () => {
    render(<TestComponent />);

    expect(subscribeMock).toHaveBeenCalledTimes(1);
    expect(subscribeMock.mock.calls[0][0]).toBe('shell_spog_mfe_operator_events');
    expect(subscribeMock.mock.calls[0][1]).toBe(`${window.location.origin}/mfe_operator_events`);
    expect(typeof subscribeMock.mock.calls[0][2]).toBe('function');
  });

  it('unwraps envelope payloads and filters only relevant event types', () => {
    render(<TestComponent />);

    const handler = subscribeMock.mock.calls[0][2] as (data: string) => void;

    act(() => {
      handler(JSON.stringify({ event: { event_type: 'somethingElse', event_origin: 'mfe-operator' } }));
    });

    expect(screen.getByTestId('event_type')).toHaveTextContent('');

    act(() => {
      handler(
        JSON.stringify({
          event: { event_type: 'ui_refresh', event_origin: 'mfe-operator', timestamp: '1' },
        })
      );
    });

    expect(screen.getByTestId('event_type')).toHaveTextContent('ui_refresh');
    expect(screen.getByTestId('origin')).toHaveTextContent('mfe-operator');
  });

  it('ignores empty and non-JSON payloads', async () => {
    render(<TestComponent />);

    const handler = subscribeMock.mock.calls[0][2] as (data: string) => Promise<void>;

    await act(async () => {
      await handler('');
      await handler('not-json');
    });

    expect(screen.getByTestId('event_type')).toHaveTextContent('');
  });

  it('accepts non-envelope payloads when they match the event type', async () => {
    render(<TestComponent />);

    const handler = subscribeMock.mock.calls[0][2] as (data: string) => Promise<void>;

    await act(async () => {
      await handler(JSON.stringify({ event_type: 'ui_refresh', event_origin: 'mfe-operator', timestamp: '1' }));
    });

    expect(screen.getByTestId('event_type')).toHaveTextContent('ui_refresh');
  });

  it('emits a distinct object reference for each accepted event message', async () => {
    const seen: any[] = [];

    function Capture() {
      const { event } = useMfeOperatorConfigSse();
      React.useEffect(() => {
        if (event) seen.push(event);
      }, [event]);
      return null;
    }

    render(<Capture />);

    const handler = subscribeMock.mock.calls[0][2] as (data: string) => Promise<void>;
    const payload = JSON.stringify({ event: { event_type: 'ui_refresh', event_origin: 'mfe-operator', timestamp: '1' } });

    // React 18 batches state updates within the same tick.
    // Use separate act() calls to force two distinct commits.
    await act(async () => {
      await handler(payload);
    });

    await act(async () => {
      await handler(payload);
    });

    expect(seen).toHaveLength(2);
    expect(seen[0]).not.toBe(seen[1]);
  });

  it('unsubscribes on unmount', () => {
    const { unmount } = render(<TestComponent />);
    unmount();

    expect(unsubscribeMock).toHaveBeenCalledWith('shell_spog_mfe_operator_events');
  });
});
