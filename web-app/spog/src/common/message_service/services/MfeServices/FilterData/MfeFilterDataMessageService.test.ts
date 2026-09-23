import { MfeFilterDataMessageService } from './MfeFilterDataMessageService';
import { FilterDataMessageService } from '../../MessageService/FilterDataMessageService/FilterDataMessageService';
import * as FilterDataServiceModule from '../../DomainService/FilterDataService';

jest.mock('../../MessageService/FilterDataMessageService/FilterDataMessageService');
jest.mock('../../DomainService/FilterDataService');

// Type for resetting the singleton
type MfeFilterDataServiceType = typeof MfeFilterDataMessageService & {
    [key: string]: any;
};

describe('MfeFilterDataMessageService', () => {
    let mockFilterDataMessageService: jest.Mocked<FilterDataMessageService>;
    let mockGetFilterDataService: jest.MockedFunction<typeof FilterDataServiceModule.getFilterDataService>;
    let addEventListenerSpy: jest.SpyInstance;
    let removeEventListenerSpy: jest.SpyInstance;

    beforeEach(() => {
        jest.clearAllMocks();

        mockFilterDataMessageService = {
            publishInitEvent: jest.fn(),
            publishUpdateEvent: jest.fn(),
            getInstance: jest.fn(),
        } as unknown as jest.Mocked<FilterDataMessageService>;

        (FilterDataMessageService.getInstance as jest.Mock).mockReturnValue(
            mockFilterDataMessageService
        );

        mockGetFilterDataService = FilterDataServiceModule.getFilterDataService as jest.MockedFunction<
            typeof FilterDataServiceModule.getFilterDataService
        >;

        addEventListenerSpy = jest.spyOn(window, 'addEventListener');
        removeEventListenerSpy = jest.spyOn(window, 'removeEventListener');
    });

    afterEach(() => {
        addEventListenerSpy.mockRestore();
        removeEventListenerSpy.mockRestore();
        // Reset singleton instance using type-safe approach
        const MfeServiceType = MfeFilterDataMessageService as MfeFilterDataServiceType;
        MfeServiceType['instance'] = null;
    });

    describe('getInstance', () => {
        it('should return the same instance on multiple calls', () => {
            const instance1 = MfeFilterDataMessageService.getInstance();
            const instance2 = MfeFilterDataMessageService.getInstance();

            expect(instance1).toBe(instance2);
        });

        it('should create a new instance on first call', () => {
            const instance = MfeFilterDataMessageService.getInstance();

            expect(instance).toBeInstanceOf(MfeFilterDataMessageService);
        });
    });

    describe('subscribe', () => {
        it('should add event listeners for both update and init events', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:update', handler);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:init', handler);
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
        });

        it('should return an unsubscribe function', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            const unsubscribe = instance.subscribe('TestComponent', handler);

            expect(typeof unsubscribe).toBe('function');
        });

        it('should remove event listeners when unsubscribe is called', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            const unsubscribe = instance.subscribe('TestComponent', handler);
            unsubscribe();

            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:update', handler);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:init', handler);
        });

        it('should call publishInitEvent when current state exists', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            const mockState = { filterId: 'filter-123', filterName: 'Test Filter' };
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(mockState),
            } as any);

            instance.subscribe('TestComponent', handler);

            expect(mockFilterDataMessageService.publishInitEvent).toHaveBeenCalledWith(mockState);
        });

        it('should not call publishInitEvent when current state is null', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            expect(mockFilterDataMessageService.publishInitEvent).not.toHaveBeenCalled();
        });

        it('should not call publishInitEvent when current state is undefined', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(undefined),
            } as any);

            instance.subscribe('TestComponent', handler);

            expect(mockFilterDataMessageService.publishInitEvent).not.toHaveBeenCalled();
        });

        it('should pass componentName parameter', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            const componentName = 'MyFilterComponent';
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe(componentName, handler);

            // Verify the method was called (componentName is accepted)
            expect(addEventListenerSpy).toHaveBeenCalled();
        });
    });

    describe('publish', () => {
        it('should call publishUpdateEvent with provided data', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const testData = { filterId: '456', filterName: 'Another Filter' };

            instance.publish(testData);

            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(testData);
        });

        it('should handle any data type', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const testString = 'filter data';

            instance.publish(testString);

            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(testString);
        });

        it('should call publishUpdateEvent multiple times', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const data1 = { id: 'filter-1' };
            const data2 = { id: 'filter-2' };

            instance.publish(data1);
            instance.publish(data2);

            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenCalledTimes(2);
            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenNthCalledWith(1, data1);
            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenNthCalledWith(2, data2);
        });

        it('should handle empty objects', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const emptyData = {};

            instance.publish(emptyData);

            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(emptyData);
        });
    });

    describe('Event Types', () => {
        it('should use correct update event type', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            const updateEventCall = addEventListenerSpy.mock.calls.find(
                (call) => call[0] === 'trading-agent:filter:update'
            );
            expect(updateEventCall).toBeDefined();
        });

        it('should use correct init event type', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            const initEventCall = addEventListenerSpy.mock.calls.find(
                (call) => call[0] === 'trading-agent:filter:init'
            );
            expect(initEventCall).toBeDefined();
        });

        it('should not mix event types between update and init', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            const calls = addEventListenerSpy.mock.calls;
            expect(calls).toHaveLength(2);
            expect(calls[0][0]).toBe('trading-agent:filter:update');
            expect(calls[1][0]).toBe('trading-agent:filter:init');
        });
    });

    describe('Integration', () => {
        it('should handle subscribe and publish in sequence', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();
            const initialState = { filterId: '123' };
            const updateData = { filterId: '123', updated: true };

            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(initialState),
            } as any);

            instance.subscribe('TestComponent', handler);
            instance.publish(updateData);

            expect(mockFilterDataMessageService.publishInitEvent).toHaveBeenCalledWith(initialState);
            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(updateData);
        });

        it('should allow multiple subscriptions', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('Component1', handler1);
            instance.subscribe('Component2', handler2);

            expect(addEventListenerSpy).toHaveBeenCalledTimes(4);
        });

        it('should allow unsubscribe and re-subscribe', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();

            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            const unsubscribe1 = instance.subscribe('Component1', handler);
            unsubscribe1();

            const unsubscribe2 = instance.subscribe('Component2', handler);

            expect(removeEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(addEventListenerSpy).toHaveBeenCalledTimes(4);
        });

        it('should handle publish without prior subscription', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const testData = { id: 'test' };

            instance.publish(testData);

            expect(mockFilterDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(testData);
        });

        it('should maintain singleton state across operations', () => {
            const instance1 = MfeFilterDataMessageService.getInstance();
            const handler1 = jest.fn();

            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue({ id: '1' }),
            } as any);

            instance1.subscribe('Comp1', handler1);

            const instance2 = MfeFilterDataMessageService.getInstance();
            expect(instance1).toBe(instance2);
            expect(mockFilterDataMessageService.publishInitEvent).toHaveBeenCalledTimes(1);
        });
    });

    describe('Error handling', () => {
        it('should handle getFilterDataService returning null', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handler = jest.fn();

            mockGetFilterDataService.mockReturnValue(null as any);

            expect(() => {
                instance.subscribe('TestComponent', handler);
            }).toThrow();
        });

        it('should handle multiple rapid subscriptions', () => {
            const instance = MfeFilterDataMessageService.getInstance();
            const handlers = [jest.fn(), jest.fn(), jest.fn()];

            mockGetFilterDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            handlers.forEach((handler) => {
                instance.subscribe('TestComponent', handler);
            });

            expect(addEventListenerSpy).toHaveBeenCalledTimes(6);
        });
    });
});
