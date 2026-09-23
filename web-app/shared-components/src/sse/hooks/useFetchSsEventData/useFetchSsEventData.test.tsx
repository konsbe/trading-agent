import { renderHook, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { useFetchEventData } from './index';
import { sseService } from '../../services/SseService';
import { UseFetchEventDataProps } from './types';

// Mock the sseService
jest.mock('../../services/SseService', () => ({
    sseService: {
        subscribe: jest.fn(),
        unsubscribe: jest.fn(),
    },
}));

const mockSseService = sseService as jest.Mocked<typeof sseService>;

describe('useFetchEventData', () => {
    let originalConsoleLog: typeof console.log;
    let originalConsoleError: typeof console.error;
    const consoleLogMock = jest.fn();
    const consoleErrorMock = jest.fn();

    beforeEach(() => {
        jest.clearAllMocks();

        // Mock console methods
        originalConsoleLog = console.log;
        originalConsoleError = console.error;
        console.log = consoleLogMock;
        console.error = consoleErrorMock;
    });

    afterEach(() => {
        console.log = originalConsoleLog;
        console.error = originalConsoleError;
    });

    describe('Initial State', () => {
        test('returns initial state with default loading true', () => {
            const mockDataFetcher = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            expect(result.current.eventData).toBeNull();
            expect(result.current.eventError).toBeNull();
            expect(result.current.isEventLoading).toBe(true);
        });

        test('returns initial state with custom initialLoading false', () => {
            const mockDataFetcher = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                initialLoading: false,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            expect(result.current.eventData).toBeNull();
            expect(result.current.eventError).toBeNull();
            expect(result.current.isEventLoading).toBe(false);
        });
    });

    describe('SSE Subscription', () => {
        test('subscribes to SSE service on mount', () => {
            const mockDataFetcher = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            renderHook(() => useFetchEventData(props));

            expect(mockSseService.subscribe).toHaveBeenCalledTimes(1);
            expect(mockSseService.subscribe).toHaveBeenCalledWith(
                'test-event',
                '/events',
                expect.any(Function)
            );
        });

        test('subscribes without logging (console.log is commented out)', () => {
            const mockDataFetcher = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'my-event',
                sseEndpoint: '/sse',
                dataFetcher: mockDataFetcher,
            };

            renderHook(() => useFetchEventData(props));

            // Implementation has console.log commented out
            expect(mockSseService.subscribe).toHaveBeenCalledWith(
                'my-event',
                '/sse',
                expect.any(Function)
            );
        });

        test('passes correct handler to sseService.subscribe', () => {
            const mockDataFetcher = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            renderHook(() => useFetchEventData(props));

            const subscribeCalls = (mockSseService.subscribe as jest.Mock).mock.calls;
            expect(subscribeCalls[0][2]).toBeInstanceOf(Function);
        });
    });

    describe('Event Handling', () => {
        test('calls dataFetcher when SSE event is received', async () => {
            const mockData = { id: 1, name: 'Test Data' };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            renderHook(() => useFetchEventData(props));

            // Get the handler passed to subscribe
            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];

            // Trigger the SSE event
            await handler('event-data');

            await waitFor(() => {
                expect(mockDataFetcher).toHaveBeenCalledTimes(1);
            });
        });

        test('handles event without logging (console.log is commented out)', async () => {
            const mockData = { id: 1, name: 'Test Data' };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-payload');

            // Implementation has console.log commented out, so verify data fetch instead
            await waitFor(() => {
                expect(mockDataFetcher).toHaveBeenCalledTimes(1);
            });
        });

        test('sets loading state during data fetching', async () => {
            let resolveDataFetcher: (value: any) => void;
            const dataFetcherPromise = new Promise((resolve) => {
                resolveDataFetcher = resolve;
            });
            const mockDataFetcher = jest.fn(() => dataFetcherPromise);

            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                initialLoading: false,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            expect(result.current.isEventLoading).toBe(false);

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];

            // Trigger event
            const handlerPromise = handler('event-data');

            await waitFor(() => {
                expect(result.current.isEventLoading).toBe(true);
            });

            // Resolve the data fetcher
            resolveDataFetcher!({ data: 'test' });
            await handlerPromise;

            await waitFor(() => {
                expect(result.current.isEventLoading).toBe(false);
            });
        });
    });

    describe('Successful Data Fetching', () => {
        test('sets eventData on successful fetch', async () => {
            const mockData = { id: 1, name: 'Test Data' };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventData).toEqual(mockData);
                expect(result.current.eventError).toBeNull();
                expect(result.current.isEventLoading).toBe(false);
            });
        });

        test('calls onSuccess callback with fetched data', async () => {
            const mockData = { id: 1, name: 'Test Data' };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const onSuccessMock = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                onSuccess: onSuccessMock,
            };

            renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(onSuccessMock).toHaveBeenCalledTimes(1);
                expect(onSuccessMock).toHaveBeenCalledWith(mockData);
            });
        });

        test('handles generic typed data correctly', async () => {
            interface CustomData {
                userId: string;
                timestamp: number;
            }

            const mockData: CustomData = { userId: 'user123', timestamp: Date.now() };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const props: UseFetchEventDataProps<CustomData> = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData<CustomData>(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventData).toEqual(mockData);
                expect(result.current.eventData?.userId).toBe('user123');
            });
        });

        test('clears error on successful fetch after previous error', async () => {
            const mockDataFetcher = jest.fn()
                .mockRejectedValueOnce(new Error('First error'))
                .mockResolvedValueOnce({ data: 'success' });

            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];

            // First event - error
            await handler('event1');

            await waitFor(() => {
                expect(result.current.eventError).toBe('First error');
            });

            // Second event - success
            await handler('event2');

            await waitFor(() => {
                expect(result.current.eventData).toEqual({ data: 'success' });
                expect(result.current.eventError).toBeNull();
            });
        });
    });

    describe('Error Handling', () => {
        test('sets error state when dataFetcher throws Error', async () => {
            const mockError = new Error('Fetch failed');
            const mockDataFetcher = jest.fn().mockRejectedValue(mockError);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventError).toBe('Fetch failed');
                expect(result.current.isEventLoading).toBe(false);
            });
        });

        test('calls onError callback with error message', async () => {
            const mockError = new Error('Fetch failed');
            const mockDataFetcher = jest.fn().mockRejectedValue(mockError);
            const onErrorMock = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                onError: onErrorMock,
            };

            renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(onErrorMock).toHaveBeenCalledTimes(1);
                expect(onErrorMock).toHaveBeenCalledWith('Fetch failed');
            });
        });

        test('handles error without logging (console.error is commented out)', async () => {
            const mockError = new Error('Fetch failed');
            const mockDataFetcher = jest.fn().mockRejectedValue(mockError);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            // Implementation has console.error commented out, so verify error state instead
            await waitFor(() => {
                expect(result.current.eventError).toBe('Fetch failed');
            });
        });

        test('handles non-Error thrown values', async () => {
            const mockDataFetcher = jest.fn().mockRejectedValue('String error');
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventError).toBe('String error');
            });
        });

        test('handles null/undefined errors', async () => {
            const mockDataFetcher = jest.fn().mockRejectedValue(null);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventError).toBe('null');
            });
        });

        test('does not call onSuccess when error occurs', async () => {
            const mockError = new Error('Fetch failed');
            const mockDataFetcher = jest.fn().mockRejectedValue(mockError);
            const onSuccessMock = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                onSuccess: onSuccessMock,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventError).toBe('Fetch failed');
            });

            expect(onSuccessMock).not.toHaveBeenCalled();
        });
    });

    describe('Multiple Events', () => {
        test('handles multiple sequential events', async () => {
            const mockDataFetcher = jest.fn()
                .mockResolvedValueOnce({ count: 1 })
                .mockResolvedValueOnce({ count: 2 })
                .mockResolvedValueOnce({ count: 3 });

            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];

            await handler('event1');
            await waitFor(() => {
                expect(result.current.eventData).toEqual({ count: 1 });
            });

            await handler('event2');
            await waitFor(() => {
                expect(result.current.eventData).toEqual({ count: 2 });
            });

            await handler('event3');
            await waitFor(() => {
                expect(result.current.eventData).toEqual({ count: 3 });
            });

            expect(mockDataFetcher).toHaveBeenCalledTimes(3);
        });

        test('maintains correct state through success and error events', async () => {
            const mockDataFetcher = jest.fn()
                .mockResolvedValueOnce({ status: 'ok' })
                .mockRejectedValueOnce(new Error('Failed'))
                .mockResolvedValueOnce({ status: 'recovered' });

            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];

            // First event - success
            await handler('event1');
            await waitFor(() => {
                expect(result.current.eventData).toEqual({ status: 'ok' });
                expect(result.current.eventError).toBeNull();
            });

            // Second event - error
            await handler('event2');
            await waitFor(() => {
                expect(result.current.eventError).toBe('Failed');
            });

            // Third event - success again
            await handler('event3');
            await waitFor(() => {
                expect(result.current.eventData).toEqual({ status: 'recovered' });
                expect(result.current.eventError).toBeNull();
            });
        });
    });

    describe('Callback Behavior', () => {
        test('works without optional callbacks', async () => {
            const mockData = { id: 1 };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                // No onSuccess or onError
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];

            // Should not throw
            await expect(handler('event-data')).resolves.not.toThrow();

            await waitFor(() => {
                expect(result.current.eventData).toEqual(mockData);
            });
        });

        test('calls only onSuccess when no error occurs', async () => {
            const mockData = { id: 1 };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const onSuccessMock = jest.fn();
            const onErrorMock = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                onSuccess: onSuccessMock,
                onError: onErrorMock,
            };

            renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(onSuccessMock).toHaveBeenCalledTimes(1);
                expect(onErrorMock).not.toHaveBeenCalled();
            });
        });

        test('calls only onError when error occurs', async () => {
            const mockError = new Error('Failed');
            const mockDataFetcher = jest.fn().mockRejectedValue(mockError);
            const onSuccessMock = jest.fn();
            const onErrorMock = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
                onSuccess: onSuccessMock,
                onError: onErrorMock,
            };

            renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(onErrorMock).toHaveBeenCalledTimes(1);
                expect(onSuccessMock).not.toHaveBeenCalled();
            });
        });
    });

    describe('Edge Cases', () => {
        test('handles empty string event data', async () => {
            const mockData = { result: 'ok' };
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('');

            await waitFor(() => {
                expect(result.current.eventData).toEqual(mockData);
            });
        });

        test('handles special characters in eventName', () => {
            const mockDataFetcher = jest.fn();
            const specialEventName = 'event-@#$%_special';
            const props: UseFetchEventDataProps = {
                eventName: specialEventName,
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            renderHook(() => useFetchEventData(props));

            expect(mockSseService.subscribe).toHaveBeenCalledWith(
                specialEventName,
                expect.any(String),
                expect.any(Function)
            );
        });

        test('handles very long event names', () => {
            const mockDataFetcher = jest.fn();
            const longEventName = 'event-' + 'a'.repeat(1000);
            const props: UseFetchEventDataProps = {
                eventName: longEventName,
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            renderHook(() => useFetchEventData(props));

            // Verify subscription with long event name
            expect(mockSseService.subscribe).toHaveBeenCalledWith(
                longEventName,
                '/events',
                expect.any(Function)
            );
        });

        test('handles dataFetcher that returns null', async () => {
            const mockDataFetcher = jest.fn().mockResolvedValue(null);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventData).toBeNull();
                expect(result.current.eventError).toBeNull();
            });
        });

        test('handles dataFetcher that returns undefined', async () => {
            const mockDataFetcher = jest.fn().mockResolvedValue(undefined);
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventData).toBeUndefined();
                expect(result.current.eventError).toBeNull();
            });
        });

        test('handles array data types', async () => {
            const mockData = [1, 2, 3, 4, 5];
            const mockDataFetcher = jest.fn().mockResolvedValue(mockData);
            const props: UseFetchEventDataProps<number[]> = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData<number[]>(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventData).toEqual([1, 2, 3, 4, 5]);
            });
        });

        test('handles primitive data types', async () => {
            const mockDataFetcher = jest.fn().mockResolvedValue('simple string');
            const props: UseFetchEventDataProps<string> = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { result } = renderHook(() => useFetchEventData<string>(props));

            const handler = (mockSseService.subscribe as jest.Mock).mock.calls[0][2];
            await handler('event-data');

            await waitFor(() => {
                expect(result.current.eventData).toBe('simple string');
            });
        });
    });

    describe('Dependency Changes', () => {
        test('resubscribes when eventName changes', () => {
            const mockDataFetcher = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'event1',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { rerender } = renderHook<{ eventName: string }, any>(
                ({ eventName }) => useFetchEventData({ ...props, eventName }),
                { initialProps: { eventName: 'event1' } }
            );

            expect(mockSseService.subscribe).toHaveBeenCalledTimes(1);
            expect(mockSseService.subscribe).toHaveBeenCalledWith(
                'event1',
                expect.any(String),
                expect.any(Function)
            );

            rerender({ eventName: 'event2' });

            expect(mockSseService.subscribe).toHaveBeenCalledTimes(2);
            expect(mockSseService.subscribe).toHaveBeenLastCalledWith(
                'event2',
                expect.any(String),
                expect.any(Function)
            );
        });

        test('resubscribes when sseEndpoint changes', () => {
            const mockDataFetcher = jest.fn();
            const props: UseFetchEventDataProps = {
                eventName: 'test-event',
                sseEndpoint: '/events',
                dataFetcher: mockDataFetcher,
            };

            const { rerender } = renderHook<{ sseEndpoint: string }, any>(
                ({ sseEndpoint }) => useFetchEventData({ ...props, sseEndpoint }),
                { initialProps: { sseEndpoint: '/events' } }
            );

            expect(mockSseService.subscribe).toHaveBeenCalledTimes(1);

            rerender({ sseEndpoint: '/new-events' });

            expect(mockSseService.subscribe).toHaveBeenCalledTimes(2);
            expect(mockSseService.subscribe).toHaveBeenLastCalledWith(
                expect.any(String),
                '/new-events',
                expect.any(Function)
            );
        });
    });
});
