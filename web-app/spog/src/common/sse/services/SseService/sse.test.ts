describe('SseService', () => {
  beforeEach(() => {
    jest.resetModules();
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('subscribes once per eventName and forwards messages', async () => {
    const instances: any[] = [];
    const EventSourceMock = jest.fn((url: string) => {
      const es: any = {
        url,
        close: jest.fn(),
        onopen: null,
        onmessage: null,
        onerror: null,
      };
      instances.push(es);
      return es;
    });

    (global as any).EventSource = EventSourceMock;

  const infoSpy = jest.spyOn(console, 'info').mockImplementation(() => undefined);

    const { SseService } = await import('./sse');
    const sse = SseService.getInstance();

    const onRcv = jest.fn();

    sse.subscribe('evt', 'http://example/sse', onRcv);
    sse.subscribe('evt', 'http://example/sse', onRcv);

    expect(EventSourceMock).toHaveBeenCalledTimes(1);
    expect(instances[0].url).toBe('http://example/sse');

    // simulate open
    instances[0].onopen?.(new Event('open'));

    // simulate message
    instances[0].onmessage?.({ data: 'hello' });
    expect(onRcv).toHaveBeenCalledWith('hello');

    sse.unsubscribe('evt');
    expect(instances[0].close).toHaveBeenCalledTimes(1);

    infoSpy.mockRestore();
  });

  it('no-ops when subscribing with empty args', async () => {
    const EventSourceMock = jest.fn();
    (global as any).EventSource = EventSourceMock;

    const warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => undefined);

    const { SseService } = await import('./sse');
    const sse = SseService.getInstance();

    sse.subscribe('', 'http://example/sse', () => undefined);
    sse.subscribe('evt', '', () => undefined);

    expect(EventSourceMock).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalled();

    warnSpy.mockRestore();
  });

  it('no-ops when unsubscribing an unknown eventName', async () => {
    const EventSourceMock = jest.fn(() => ({ close: jest.fn() }));
    (global as any).EventSource = EventSourceMock;

    const { SseService } = await import('./sse');
    const sse = SseService.getInstance();

    expect(() => sse.unsubscribe('missing')).not.toThrow();
    expect(EventSourceMock).not.toHaveBeenCalled();
  });

  it('retries on error with exponential backoff and stops after max retries', async () => {
    const instances: any[] = [];
    const EventSourceMock = jest.fn((url: string) => {
      const es: any = {
        url,
        close: jest.fn(),
        onopen: null,
        onmessage: null,
        onerror: null,
      };
      instances.push(es);
      return es;
    });

    (global as any).EventSource = EventSourceMock;

    const warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => undefined);
    const infoSpy = jest.spyOn(console, 'info').mockImplementation(() => undefined);

    const { SseService } = await import('./sse');
    const sse = SseService.getInstance();

    sse.subscribe('evt', 'http://example/sse', () => undefined);
    expect(EventSourceMock).toHaveBeenCalledTimes(1);

    // Trigger errors until max retries is reached.
    // MAX_RETRIES is 5; once exceeded, the entry is removed.
    for (let i = 0; i < 6; i++) {
      const current = instances[instances.length - 1];
      current.onerror?.(new Event('error'));
      // run the scheduled reconnect, if any
      jest.runOnlyPendingTimers();
    }

    // Should have attempted multiple reconnects, but not infinite.
    expect(EventSourceMock.mock.calls.length).toBeGreaterThan(1);
    expect(warnSpy).toHaveBeenCalled();

    warnSpy.mockRestore();
    infoSpy.mockRestore();
  });

  it('uses expected retry delays (including max clamp) and clears any existing retry timer', async () => {
    const instances: any[] = [];
    const EventSourceMock = jest.fn((url: string) => {
      const es: any = {
        url,
        close: jest.fn(),
        onopen: null,
        onmessage: null,
        onerror: null,
      };
      instances.push(es);
      return es;
    });

    (global as any).EventSource = EventSourceMock;

    const warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => undefined);
    const infoSpy = jest.spyOn(console, 'info').mockImplementation(() => undefined);
    const setTimeoutSpy = jest.spyOn(window, 'setTimeout');
    const clearTimeoutSpy = jest.spyOn(window, 'clearTimeout');

    const { SseService } = await import('./sse');
    const sse = SseService.getInstance();

    sse.subscribe('evt', 'http://example/sse', () => undefined);

    // Fire an error twice without letting the timer execute; the second should clear the first timer.
    instances[0].onerror?.(new Event('error'));
    instances[0].onerror?.(new Event('error'));

    expect(clearTimeoutSpy).toHaveBeenCalled();

    // Retry delays: 3000, 6000, 12000, 24000, 30000 (clamped)
    const delays = setTimeoutSpy.mock.calls.map((c) => c[1]);
    expect(delays).toContain(3000);
    expect(delays).toContain(6000);

    // Force more retries to reach the clamp.
    for (let i = 0; i < 4; i++) {
      const current = instances[instances.length - 1];
      current.onerror?.(new Event('error'));
      jest.runOnlyPendingTimers();
    }

    const allDelays = setTimeoutSpy.mock.calls.map((c) => c[1]);
    expect(allDelays).toContain(30000);

    warnSpy.mockRestore();
    infoSpy.mockRestore();
    setTimeoutSpy.mockRestore();
    clearTimeoutSpy.mockRestore();
  });

  it('does not throw if onRcvHandler throws', async () => {
    const instances: any[] = [];
    const EventSourceMock = jest.fn((url: string) => {
      const es: any = {
        url,
        close: jest.fn(),
        onopen: null,
        onmessage: null,
        onerror: null,
      };
      instances.push(es);
      return es;
    });

    (global as any).EventSource = EventSourceMock;

    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => undefined);

    const { SseService } = await import('./sse');
    const sse = SseService.getInstance();

    sse.subscribe('evt', 'http://example/sse', () => {
      throw new Error('boom');
    });

    expect(() => {
      instances[0].onmessage?.({ data: 'x' });
    }).not.toThrow();

    expect(errorSpy).toHaveBeenCalled();
    errorSpy.mockRestore();
  });
});
