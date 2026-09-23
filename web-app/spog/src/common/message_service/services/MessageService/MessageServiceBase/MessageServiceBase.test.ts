import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import MessageServiceBase from '.';

// Mock the MfeOrchestrationService
jest.mock('../../MfeOrchestrationService', () => ({
    __esModule: true,
    default: {
        registerMount: jest.fn(),
    },
}));

import mfeOrchestrationService from '../../MfeOrchestrationService';

describe('MessageServiceBase', () => {
    let messageService: MessageServiceBase;
    const testDataDomain = 'testDomain';
    let mockCallback: jest.Mock;
    let dispatchEventSpy: jest.SpyInstance;
    let addEventListenerSpy: jest.SpyInstance;
    let removeEventListenerSpy: jest.SpyInstance;

    beforeEach(() => {
        // Create a new instance for each test
        messageService = new MessageServiceBase(testDataDomain);
        
        // Create mock callback
        mockCallback = jest.fn();
        
        // Spy on window methods
        dispatchEventSpy = jest.spyOn(window, 'dispatchEvent');
        addEventListenerSpy = jest.spyOn(window, 'addEventListener');
        removeEventListenerSpy = jest.spyOn(window, 'removeEventListener');
        
        // Clear all mocks
        jest.clearAllMocks();
        jest.useFakeTimers();
    });

    afterEach(() => {
        jest.clearAllTimers();
        jest.useRealTimers();
        dispatchEventSpy.mockRestore();
        addEventListenerSpy.mockRestore();
        removeEventListenerSpy.mockRestore();
    });

    describe('Constructor', () => {
        it('should create an instance with correct data domain', () => {
            expect(messageService.dataDomain).toBe(testDataDomain);
        });

        it('should initialize event type properties correctly', () => {
            expect(messageService.eventTypeApp).toBe('trading-agent');
            expect(messageService.initAction).toBe('init');
            expect(messageService.updateAction).toBe('update');
        });

        it('should create correct update event type', () => {
            expect(messageService.updateEventType).toBe('trading-agent:testDomain:update');
        });

        it('should create correct init event type', () => {
            expect(messageService.initEventType).toBe('trading-agent:testDomain:init');
        });

        it('should set timer value correctly', () => {
            expect(messageService.removeInitEventListenerTimerInSeconds).toBe(2);
        });

        it('should return the instance', () => {
            const instance = new MessageServiceBase('anotherDomain');
            expect(instance).toBeInstanceOf(MessageServiceBase);
        });

        it('should handle different data domain names', () => {
            const service1 = new MessageServiceBase('user');
            const service2 = new MessageServiceBase('filter');
            
            expect(service1.dataDomain).toBe('user');
            expect(service2.dataDomain).toBe('filter');
            expect(service1.updateEventType).toBe('trading-agent:user:update');
            expect(service2.updateEventType).toBe('trading-agent:filter:update');
        });
    });

    describe('publishUpdateEvent', () => {
        it('should create and dispatch update event with detail', async () => {
            const testDetail = { data: 'test data' };
            
            messageService.publishUpdateEvent(testDetail);
            
            // Fast-forward time
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.type).toBe('trading-agent:testDomain:update');
            expect(dispatchedEvent.detail).toEqual(testDetail);
        });

        it('should dispatch event asynchronously', () => {
            const testDetail = { value: 123 };
            
            messageService.publishUpdateEvent(testDetail);
            
            // Should not be called immediately
            expect(dispatchEventSpy).not.toHaveBeenCalled();
            
            // Fast-forward time
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
        });

        it('should handle null detail', () => {
            messageService.publishUpdateEvent(null);
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toBeNull();
        });

        it('should handle undefined detail', () => {
            messageService.publishUpdateEvent(undefined);
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            // CustomEvent converts undefined to null
            expect(dispatchedEvent.detail).toBeNull();
        });

        it('should handle complex object as detail', () => {
            const complexDetail = {
                nested: {
                    data: [1, 2, 3],
                    obj: { key: 'value' }
                },
                array: ['a', 'b', 'c']
            };
            
            messageService.publishUpdateEvent(complexDetail);
            
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(complexDetail);
        });

        it('should dispatch multiple events in sequence', () => {
            messageService.publishUpdateEvent({ id: 1 });
            messageService.publishUpdateEvent({ id: 2 });
            messageService.publishUpdateEvent({ id: 3 });
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(3);
        });
    });

    describe('publishInitEvent', () => {
        it('should create and dispatch init event with detail', () => {
            const testDetail = { initial: 'data' };
            
            messageService.publishInitEvent(testDetail);
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.type).toBe('trading-agent:testDomain:init');
            expect(dispatchedEvent.detail).toEqual(testDetail);
        });

        it('should dispatch event asynchronously', () => {
            const testDetail = { init: true };
            
            messageService.publishInitEvent(testDetail);
            
            // Should not be called immediately
            expect(dispatchEventSpy).not.toHaveBeenCalled();
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
        });

        it('should handle empty object as detail', () => {
            messageService.publishInitEvent({});
            
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual({});
        });

        it('should handle array as detail', () => {
            const arrayDetail = [1, 2, 3, 4, 5];
            
            messageService.publishInitEvent(arrayDetail);
            
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(arrayDetail);
        });

        it('should dispatch both init and update events independently', () => {
            messageService.publishInitEvent({ type: 'init' });
            messageService.publishUpdateEvent({ type: 'update' });
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(2);
            
            const firstEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            const secondEvent = dispatchEventSpy.mock.calls[1][0] as CustomEvent;
            
            expect(firstEvent.type).toBe('trading-agent:testDomain:init');
            expect(secondEvent.type).toBe('trading-agent:testDomain:update');
        });
    });

    describe('subscribe', () => {
        it('should subscribe to update and init events', () => {
            const cleanup = messageService.subscribe('testComponent', mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:update', mockCallback);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:init', mockCallback);
        });

        it('should register mount with orchestration service', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            expect(mfeOrchestrationService.registerMount).toHaveBeenCalledTimes(1);
        });

        it('should return cleanup function', () => {
            const cleanup = messageService.subscribe('testComponent', mockCallback);
            
            expect(typeof cleanup).toBe('function');
        });

        it('should cleanup should remove update event listener', () => {
            const cleanup = messageService.subscribe('testComponent', mockCallback);
            
            cleanup();
            
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:update', mockCallback);
        });

        it('should cleanup should remove init event listener', () => {
            const cleanup = messageService.subscribe('testComponent', mockCallback);
            
            cleanup();
            
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:init', mockCallback);
        });

        it('should receive update events after subscription', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            const testDetail = { message: 'test' };
            messageService.publishUpdateEvent(testDetail);
            
            jest.runAllTimers();
            
            expect(mockCallback).toHaveBeenCalledTimes(1);
            expect(mockCallback.mock.calls[0][0].detail).toEqual(testDetail);
        });

        it('should receive init events after subscription', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            const testDetail = { initial: 'data' };
            messageService.publishInitEvent(testDetail);
            
            jest.runAllTimers();
            
            expect(mockCallback).toHaveBeenCalledTimes(1);
            expect(mockCallback.mock.calls[0][0].detail).toEqual(testDetail);
        });

        it('should stop receiving update events after cleanup', () => {
            const cleanup = messageService.subscribe('testComponent', mockCallback);
            
            cleanup();
            
            messageService.publishUpdateEvent({ data: 'test' });
            jest.runAllTimers();
            
            expect(mockCallback).not.toHaveBeenCalled();
        });

        it('should handle multiple subscriptions', () => {
            const callback1 = jest.fn();
            const callback2 = jest.fn();
            
            messageService.subscribe('component1', callback1);
            messageService.subscribe('component2', callback2);
            
            messageService.publishUpdateEvent({ data: 'broadcast' });
            jest.runAllTimers();
            
            expect(callback1).toHaveBeenCalledTimes(1);
            expect(callback2).toHaveBeenCalledTimes(1);
        });

        it('should automatically remove init listener after timeout', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            // Fast-forward past the init listener timeout (2 seconds)
            jest.advanceTimersByTime(2000);
            
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:init', mockCallback);
        });
    });

    describe('subscribeToUpdateEvent', () => {
        it('should add event listener for update event type', () => {
            messageService.subscribeToUpdateEvent(mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:update', mockCallback);
        });

        it('should receive update events', () => {
            messageService.subscribeToUpdateEvent(mockCallback);
            
            const testDetail = { update: 'data' };
            messageService.publishUpdateEvent(testDetail);
            
            jest.runAllTimers();
            
            expect(mockCallback).toHaveBeenCalledTimes(1);
        });

        it('should allow multiple callbacks for same event', () => {
            const callback1 = jest.fn();
            const callback2 = jest.fn();
            
            messageService.subscribeToUpdateEvent(callback1);
            messageService.subscribeToUpdateEvent(callback2);
            
            messageService.publishUpdateEvent({ data: 'test' });
            jest.runAllTimers();
            
            expect(callback1).toHaveBeenCalledTimes(1);
            expect(callback2).toHaveBeenCalledTimes(1);
        });
    });

    describe('removeUpdateEventListener', () => {
        it('should remove event listener for update event type', () => {
            messageService.subscribeToUpdateEvent(mockCallback);
            messageService.removeUpdateEventListener(mockCallback);
            
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:update', mockCallback);
        });

        it('should stop receiving update events after removal', () => {
            messageService.subscribeToUpdateEvent(mockCallback);
            messageService.removeUpdateEventListener(mockCallback);
            
            messageService.publishUpdateEvent({ data: 'test' });
            jest.runAllTimers();
            
            expect(mockCallback).not.toHaveBeenCalled();
        });

        it('should not affect other callbacks', () => {
            const callback1 = jest.fn();
            const callback2 = jest.fn();
            
            messageService.subscribeToUpdateEvent(callback1);
            messageService.subscribeToUpdateEvent(callback2);
            
            messageService.removeUpdateEventListener(callback1);
            
            messageService.publishUpdateEvent({ data: 'test' });
            jest.runAllTimers();
            
            expect(callback1).not.toHaveBeenCalled();
            expect(callback2).toHaveBeenCalledTimes(1);
        });
    });

    describe('Edge cases and integration', () => {
        it('should handle rapid publish and subscribe operations', () => {
            messageService.subscribe('component1', mockCallback);
            
            for (let i = 0; i < 10; i++) {
                messageService.publishUpdateEvent({ count: i });
            }
            
            jest.runAllTimers();
            
            expect(mockCallback).toHaveBeenCalledTimes(10);
        });

        it('should work with different data domain instances', () => {
            const service1 = new MessageServiceBase('domain1');
            const service2 = new MessageServiceBase('domain2');
            
            const callback1 = jest.fn();
            const callback2 = jest.fn();
            
            service1.subscribe('comp1', callback1);
            service2.subscribe('comp2', callback2);
            
            service1.publishUpdateEvent({ source: 'domain1' });
            jest.runAllTimers();
            
            expect(callback1).toHaveBeenCalledTimes(1);
            expect(callback2).not.toHaveBeenCalled();
        });

        it('should handle cleanup of multiple subscriptions', () => {
            const cleanup1 = messageService.subscribe('comp1', jest.fn());
            const cleanup2 = messageService.subscribe('comp2', jest.fn());
            const cleanup3 = messageService.subscribe('comp3', jest.fn());
            
            cleanup1();
            cleanup2();
            cleanup3();
            
            expect(removeEventListenerSpy).toHaveBeenCalledTimes(6); // 3 subscriptions × 2 event types
        });

        it('should handle subscription with same callback from different components', () => {
            const sharedCallback = jest.fn();
            
            messageService.subscribe('component1', sharedCallback);
            messageService.subscribe('component2', sharedCallback);
            
            messageService.publishUpdateEvent({ data: 'shared' });
            jest.runAllTimers();
            
            // addEventListener with same callback is only added once (browser behavior)
            expect(sharedCallback).toHaveBeenCalledTimes(1);
        });

        it('should properly isolate event types', () => {
            const updateCallback = jest.fn();
            const initCallback = jest.fn();
            
            messageService.subscribeToUpdateEvent(updateCallback);
            messageService.subscribe('testComponent', initCallback);
            
            messageService.publishUpdateEvent({ type: 'update' });
            jest.runAllTimers();
            
            expect(updateCallback).toHaveBeenCalledTimes(1);
            expect(initCallback).toHaveBeenCalledTimes(1);
        });

        it('should handle empty component name', () => {
            const cleanup = messageService.subscribe('', mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(typeof cleanup).toBe('function');
        });

        it('should handle special characters in detail', () => {
            const specialDetail = {
                text: 'Hello <script>alert("test")</script>',
                symbols: '!@#$%^&*()',
                unicode: '你好世界 🚀',
            };
            
            messageService.publishUpdateEvent(specialDetail);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(specialDetail);
        });

        it('should handle very large detail objects', () => {
            const largeDetail = {
                items: Array.from({ length: 1000 }, (_, i) => ({ id: i, value: `item-${i}` }))
            };
            
            messageService.publishUpdateEvent(largeDetail);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail.items).toHaveLength(1000);
        });
    });

    describe('Timer behavior', () => {
        it('should use 2-second timeout for init event listener removal', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            // Before timeout
            jest.advanceTimersByTime(1999);
            expect(removeEventListenerSpy).not.toHaveBeenCalledWith('trading-agent:testDomain:init', mockCallback);
            
            // After timeout
            jest.advanceTimersByTime(1);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:init', mockCallback);
        });

        it('should not remove update listener after timeout', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            jest.advanceTimersByTime(5000);
            
            // Init listener should be removed, but update listener should remain
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:testDomain:init', mockCallback);
            expect(removeEventListenerSpy).not.toHaveBeenCalledWith('trading-agent:testDomain:update', mockCallback);
        });

        it('should receive update events even after init timeout', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            jest.advanceTimersByTime(3000); // Past init timeout
            
            mockCallback.mockClear();
            
            messageService.publishUpdateEvent({ data: 'after timeout' });
            jest.runAllTimers();
            
            expect(mockCallback).toHaveBeenCalledTimes(1);
        });

        it('should not receive init events after timeout', () => {
            messageService.subscribe('testComponent', mockCallback);
            
            jest.advanceTimersByTime(3000); // Past init timeout
            
            mockCallback.mockClear();
            
            messageService.publishInitEvent({ data: 'after timeout' });
            jest.runAllTimers();
            
            expect(mockCallback).not.toHaveBeenCalled();
        });
    });
});
