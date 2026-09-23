import '@testing-library/jest-dom'

// -----------------------------
// EventSource mock
// -----------------------------
export const EventSourceMock = jest.fn((url: string) => {
    return {
        url,
        close: jest.fn(),
        onopen: null as (() => void) | null,
        onmessage: null as ((event: MessageEvent) => void) | null,
        onerror: null as (() => void) | null,
    }
})

Object.defineProperty(global, 'EventSource', {
    writable: true,
    value: EventSourceMock,
})

// -----------------------------
// Tests
// -----------------------------
describe('SseService', () => {
    let sseService: any
    const consoleLogMock = jest.fn();
    const consoleErrorMock = jest.fn();
    const consoleWarnMock = jest.fn();
    let originalConsoleLog: typeof console.log;
    let originalConsoleError: typeof console.error;
    let originalConsoleWarn: typeof console.warn;

    beforeEach(async () => {
        // Reset modules so singleton and eventSources are fresh
        jest.resetModules()
        jest.clearAllMocks()
        jest.useFakeTimers()

        // Mock console methods
        originalConsoleLog = console.log;
        originalConsoleError = console.error;
        originalConsoleWarn = console.warn;
        console.log = consoleLogMock;
        console.error = consoleErrorMock;
        console.warn = consoleWarnMock;

        // Re-import the module fresh
        const module = await import('./index')
        sseService = module.sseService
    })

    afterEach(() => {
        jest.useRealTimers()
        console.log = originalConsoleLog;
        console.error = originalConsoleError;
        console.warn = originalConsoleWarn;
    })

    test('subscribes and creates an EventSource', () => {
        const handler = jest.fn()
        sseService.subscribe('clientA', '/sse/a', handler)

        expect(EventSourceMock).toHaveBeenCalledTimes(1)
        expect(EventSourceMock).toHaveBeenCalledWith('/sse/a')
    })

    test('does not create duplicate connections for same eventName', () => {
        const handler = jest.fn()

        sseService.subscribe('clientA', '/sse/a', handler)
        sseService.subscribe('clientA', '/sse/a', handler)

        expect(EventSourceMock).toHaveBeenCalledTimes(1)
    })

    test('calls onRcvHandler when message is received', () => {
        const handler = jest.fn()

        sseService.subscribe('clientB', '/sse/b', handler)

        const instance = EventSourceMock.mock.results[0].value

        instance.onmessage?.({ data: 'payload' } as MessageEvent)

        expect(handler).toHaveBeenCalledWith('payload')
    })

    test('resets retries on successful connection (onopen)', () => {
        const handler = jest.fn()

        sseService.subscribe('clientC', '/sse/c', handler)

        const instance = EventSourceMock.mock.results[0].value

        // simulate open
        instance.onopen?.()

        // trigger error to cause retry
        instance.onerror?.()

        jest.advanceTimersByTime(3000)

        // second EventSource created
        expect(EventSourceMock).toHaveBeenCalledTimes(2)
    })

    test('retries on error after delay', () => {
        const handler = jest.fn()

        sseService.subscribe('clientD', '/sse/d', handler)

        const instance = EventSourceMock.mock.results[0].value

        instance.onerror?.()

        // not yet retried
        expect(EventSourceMock).toHaveBeenCalledTimes(1)

        jest.advanceTimersByTime(3000)

        expect(EventSourceMock).toHaveBeenCalledTimes(2)
    })

    test('closes EventSource before retrying', () => {
        const handler = jest.fn()

        sseService.subscribe('clientE', '/sse/e', handler)

        const instance = EventSourceMock.mock.results[0].value

        instance.onerror?.()

        expect(instance.close).toHaveBeenCalledTimes(1)
    })

    test('stops retrying after max retries', () => {
        const handler = jest.fn()

        sseService.subscribe('clientF', '/sse/f', handler)

        let instance = EventSourceMock.mock.results[0].value

        for (let i = 0; i < 6; i++) {
            instance.onerror?.()
            jest.advanceTimersByTime(3000)
            instance = EventSourceMock.mock.results.at(-1)?.value
        }

        // initial + 5 retries
        expect(EventSourceMock).toHaveBeenCalledTimes(6)
    })

    test('creates new EventSource instance on each retry', () => {
        const handler = jest.fn()

        sseService.subscribe('clientG', '/sse/g', handler)

        const first = EventSourceMock.mock.results[0].value
        first.onerror?.()

        jest.advanceTimersByTime(3000)

        const second = EventSourceMock.mock.results[1].value

        expect(first).not.toBe(second)
    })

    test('unsubscribes and closes connection', () => {
        const handler = jest.fn()

        sseService.subscribe('clientUnsubscribe', '/sse/unsub', handler)

        const instance = EventSourceMock.mock.results[0].value

        // Unsubscribe
        sseService.unsubscribe('clientUnsubscribe')

        expect(instance.close).toHaveBeenCalledTimes(1)
        // Implementation has console.log commented out, so just verify close was called
    })

    test('unsubscribe clears pending retry timer', () => {
        const handler = jest.fn()

        sseService.subscribe('clientTimerClear', '/sse/timer', handler)

        const instance = EventSourceMock.mock.results[0].value

        // Trigger error to schedule retry
        instance.onerror?.()

        // Unsubscribe before retry executes
        sseService.unsubscribe('clientTimerClear')

        // Advance time to where retry would have fired
        jest.advanceTimersByTime(5000)

        // Should not create new EventSource (timer was cleared)
        expect(EventSourceMock).toHaveBeenCalledTimes(1)
    })

    test('unsubscribe removes from registry', () => {
        const handler = jest.fn()

        sseService.subscribe('clientRegistry', '/sse/reg', handler)
        sseService.unsubscribe('clientRegistry')

        // Try to subscribe again - should succeed (was removed from registry)
        sseService.subscribe('clientRegistry', '/sse/reg', handler)

        // Should have created 2 EventSources (original + new subscription)
        expect(EventSourceMock).toHaveBeenCalledTimes(2)
    })

    test('unsubscribe on non-existent connection does not throw', () => {
        // Implementation has console.log commented out, just verify no error is thrown
        expect(() => {
            sseService.unsubscribe('nonExistentClient')
        }).not.toThrow()
    })

    test('multiple unsubscribes do not throw errors', () => {
        const handler = jest.fn()

        sseService.subscribe('clientMultiUnsub', '/sse/multi', handler)

        // Unsubscribe multiple times
        expect(() => {
            sseService.unsubscribe('clientMultiUnsub')
            sseService.unsubscribe('clientMultiUnsub')
            sseService.unsubscribe('clientMultiUnsub')
        }).not.toThrow()
    })

    test('getInstance returns the same singleton instance', async () => {
        const { SseService } = await import('./sse')
        const first = SseService.getInstance()
        const second = SseService.getInstance()

        expect(first).toBe(second)
    })

    test('ignores retry when connection entry was removed', () => {
        const handler = jest.fn()

        sseService.subscribe('clientMissingEntry', '/sse/missing-entry', handler)
        const instance = EventSourceMock.mock.results[0].value

        sseService.unsubscribe('clientMissingEntry')

        expect(() => instance.onerror?.()).not.toThrow()
        expect(EventSourceMock).toHaveBeenCalledTimes(1)
    })
})

