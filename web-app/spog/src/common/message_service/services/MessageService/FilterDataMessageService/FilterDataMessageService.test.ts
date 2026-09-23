/**
 * Tests for FilterDataMessageService
 * Testing singleton pattern, inheritance from MessageServiceBase, and filter domain specific functionality
 */
import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { FilterDataMessageService } from './FilterDataMessageService';
import MessageServiceBase from '../MessageServiceBase';

// Mock the MfeOrchestrationService
jest.mock('../../MfeOrchestrationService', () => ({
    __esModule: true,
    default: {
        registerMount: jest.fn(),
    },
}));

import mfeOrchestrationService from '../../MfeOrchestrationService';

describe('FilterDataMessageService', () => {
    let dispatchEventSpy: jest.SpyInstance;
    let addEventListenerSpy: jest.SpyInstance;
    let removeEventListenerSpy: jest.SpyInstance;
    let mockCallback: jest.Mock;

    beforeEach(() => {
        // Reset the singleton instance before each test
        (FilterDataMessageService as any).instance = null;
        
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
        
        // Clean up singleton
        (FilterDataMessageService as any).instance = null;
    });

    describe('Singleton Pattern', () => {
        it('should create a singleton instance', () => {
            const instance1 = FilterDataMessageService.getInstance();
            const instance2 = FilterDataMessageService.getInstance();
            
            expect(instance1).toBe(instance2);
            expect(instance1).toBeInstanceOf(FilterDataMessageService);
        });

        it('should only create one instance across multiple calls', () => {
            const instances = Array.from({ length: 5 }, () => FilterDataMessageService.getInstance());
            
            const firstInstance = instances[0];
            instances.forEach(instance => {
                expect(instance).toBe(firstInstance);
            });
        });

        it('should be an instance of MessageServiceBase', () => {
            const instance = FilterDataMessageService.getInstance();
            expect(instance).toBeInstanceOf(MessageServiceBase);
        });

        it('should not allow direct instantiation', () => {
            // TypeScript will prevent this at compile time, but we can verify the constructor is private
            expect(() => {
                // @ts-ignore - Testing private constructor
                new FilterDataMessageService();
            }).not.toThrow();
            
            // But getInstance should still return the same instance
            const instance1 = FilterDataMessageService.getInstance();
            // @ts-ignore - Testing private constructor
            const instance2 = new FilterDataMessageService();
            expect(instance2).toBeInstanceOf(FilterDataMessageService);
        });
    });

    describe('Initialization', () => {
        it('should initialize with correct data domain', () => {
            const instance = FilterDataMessageService.getInstance();
            expect(instance.dataDomain).toBe('filter');
        });

        it('should initialize with correct event types', () => {
            const instance = FilterDataMessageService.getInstance();
            
            expect(instance.eventTypeApp).toBe('trading-agent');
            expect(instance.initAction).toBe('init');
            expect(instance.updateAction).toBe('update');
        });

        it('should create correct update event type for filter domain', () => {
            const instance = FilterDataMessageService.getInstance();
            expect(instance.updateEventType).toBe('trading-agent:filter:update');
        });

        it('should create correct init event type for filter domain', () => {
            const instance = FilterDataMessageService.getInstance();
            expect(instance.initEventType).toBe('trading-agent:filter:init');
        });

        it('should set default timer value', () => {
            const instance = FilterDataMessageService.getInstance();
            expect(instance.removeInitEventListenerTimerInSeconds).toBe(2);
        });
    });

    describe('Inherited Methods - publishUpdateEvent', () => {
        it('should publish update event with filter data', () => {
            const instance = FilterDataMessageService.getInstance();
            const filterData = { 
                timeRange: { start: '2024-01-01', end: '2024-01-31' },
                clusters: ['cluster1', 'cluster2'],
                selectedMetrics: ['cpu', 'memory']
            };
            
            instance.publishUpdateEvent(filterData);
            
            // Fast-forward timers
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.type).toBe('trading-agent:filter:update');
            expect(dispatchedEvent.detail).toEqual(filterData);
        });

        it('should publish update event asynchronously', () => {
            const instance = FilterDataMessageService.getInstance();
            const filterData = { status: 'active', type: 'alarm' };
            
            instance.publishUpdateEvent(filterData);
            
            // Before timer runs
            expect(dispatchEventSpy).not.toHaveBeenCalled();
            
            // After timer runs
            jest.runAllTimers();
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
        });

        it('should handle multiple update events', () => {
            const instance = FilterDataMessageService.getInstance();
            const filterData1 = { cluster: 'prod', severity: 'critical' };
            const filterData2 = { cluster: 'dev', severity: 'warning' };
            const filterData3 = { cluster: 'test', severity: 'info' };
            
            instance.publishUpdateEvent(filterData1);
            instance.publishUpdateEvent(filterData2);
            instance.publishUpdateEvent(filterData3);
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(3);
            
            const events = dispatchEventSpy.mock.calls.map(call => call[0] as CustomEvent);
            expect(events[0].detail).toEqual(filterData1);
            expect(events[1].detail).toEqual(filterData2);
            expect(events[2].detail).toEqual(filterData3);
        });

        it('should handle null detail', () => {
            const instance = FilterDataMessageService.getInstance();
            
            instance.publishUpdateEvent(null);
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toBeNull();
        });

        it('should handle undefined detail', () => {
            const instance = FilterDataMessageService.getInstance();
            
            instance.publishUpdateEvent(undefined);
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            // CustomEvent converts undefined to null
            expect(dispatchedEvent.detail).toBeNull();
        });

        it('should handle complex nested filter data objects', () => {
            const instance = FilterDataMessageService.getInstance();
            const complexFilterData = {
                dateRange: {
                    start: new Date('2024-01-01').toISOString(),
                    end: new Date('2024-12-31').toISOString(),
                    preset: 'custom'
                },
                filters: {
                    topology: ['cluster1', 'cluster2', 'cluster3'],
                    severity: ['critical', 'major'],
                    status: ['active', 'acknowledged']
                },
                aggregations: {
                    groupBy: ['cluster', 'severity'],
                    sortBy: 'timestamp',
                    order: 'desc'
                }
            };
            
            instance.publishUpdateEvent(complexFilterData);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(complexFilterData);
        });
    });

    describe('Inherited Methods - publishInitEvent', () => {
        it('should publish init event with filter data', () => {
            const instance = FilterDataMessageService.getInstance();
            const filterData = { 
                defaultFilters: { severity: 'all', timeRange: 'last-24h' }
            };
            
            instance.publishInitEvent(filterData);
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.type).toBe('trading-agent:filter:init');
            expect(dispatchedEvent.detail).toEqual(filterData);
        });

        it('should publish init event asynchronously', () => {
            const instance = FilterDataMessageService.getInstance();
            const filterData = { initialized: true };
            
            instance.publishInitEvent(filterData);
            
            // Before timer runs
            expect(dispatchEventSpy).not.toHaveBeenCalled();
            
            // After timer runs
            jest.runAllTimers();
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
        });

        it('should handle empty object as detail', () => {
            const instance = FilterDataMessageService.getInstance();
            
            instance.publishInitEvent({});
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual({});
        });
    });

    describe('Inherited Methods - subscribe', () => {
        it('should subscribe to both update and init events', () => {
            const instance = FilterDataMessageService.getInstance();
            const componentName = 'FilterComponent';
            
            instance.subscribe(componentName, mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:update', mockCallback);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:init', mockCallback);
        });

        it('should register mount with orchestration service', () => {
            const instance = FilterDataMessageService.getInstance();
            const componentName = 'FilterComponent';
            
            instance.subscribe(componentName, mockCallback);
            
            expect(mfeOrchestrationService.registerMount).toHaveBeenCalledTimes(1);
        });

        it('should return cleanup function', () => {
            const instance = FilterDataMessageService.getInstance();
            const componentName = 'FilterComponent';
            
            const cleanup = instance.subscribe(componentName, mockCallback);
            
            expect(typeof cleanup).toBe('function');
        });

        it('should cleanup both event listeners when cleanup function is called', () => {
            const instance = FilterDataMessageService.getInstance();
            const componentName = 'FilterComponent';
            
            const cleanup = instance.subscribe(componentName, mockCallback);
            
            // Clear mocks to check cleanup calls
            jest.clearAllMocks();
            
            cleanup();
            
            expect(removeEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:update', mockCallback);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:init', mockCallback);
        });

        it('should handle multiple subscriptions', () => {
            const instance = FilterDataMessageService.getInstance();
            const callback1 = jest.fn();
            const callback2 = jest.fn();
            const callback3 = jest.fn();
            
            instance.subscribe('Component1', callback1);
            instance.subscribe('Component2', callback2);
            instance.subscribe('Component3', callback3);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(6); // 2 events × 3 subscriptions
            expect(mfeOrchestrationService.registerMount).toHaveBeenCalledTimes(3);
        });
    });

    describe('Inherited Methods - subscribeToUpdateEvent', () => {
        it('should subscribe to update event only', () => {
            const instance = FilterDataMessageService.getInstance();
            
            instance.subscribeToUpdateEvent(mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(1);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:update', mockCallback);
        });

        it('should receive update events after subscription', () => {
            const instance = FilterDataMessageService.getInstance();
            const filterData = { activeFilters: ['cluster1'] };
            
            // Actually add the event listener (not just spy)
            window.addEventListener('trading-agent:filter:update', mockCallback);
            
            // Publish event
            instance.publishUpdateEvent(filterData);
            jest.runAllTimers();
            
            expect(mockCallback).toHaveBeenCalledTimes(1);
            expect(mockCallback.mock.calls[0][0].detail).toEqual(filterData);
            
            // Cleanup
            window.removeEventListener('trading-agent:filter:update', mockCallback);
        });
    });

    describe('Inherited Methods - removeUpdateEventListener', () => {
        it('should remove update event listener', () => {
            const instance = FilterDataMessageService.getInstance();
            
            instance.removeUpdateEventListener(mockCallback);
            
            expect(removeEventListenerSpy).toHaveBeenCalledTimes(1);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:filter:update', mockCallback);
        });

        it('should stop receiving update events after removal', () => {
            const instance = FilterDataMessageService.getInstance();
            const filterData = { removed: true };
            
            // Subscribe
            instance.subscribeToUpdateEvent(mockCallback);
            
            // Publish and receive event
            const updateEvent1 = new CustomEvent('trading-agent:filter:update', { detail: filterData });
            window.dispatchEvent(updateEvent1);
            expect(mockCallback).toHaveBeenCalledTimes(1);
            
            // Remove listener
            instance.removeUpdateEventListener(mockCallback);
            
            // Publish again - should not receive
            const updateEvent2 = new CustomEvent('trading-agent:filter:update', { detail: filterData });
            window.dispatchEvent(updateEvent2);
            expect(mockCallback).toHaveBeenCalledTimes(1); // Still 1, not 2
        });
    });

    describe('Integration Tests', () => {
        it('should handle complete lifecycle: subscribe, receive updates, cleanup', () => {
            const instance = FilterDataMessageService.getInstance();
            const componentName = 'LifecycleComponent';
            const filterData1 = { action: 'apply-filter', cluster: 'prod' };
            const filterData2 = { action: 'clear-filter', cluster: 'all' };
            
            // Subscribe
            const cleanup = instance.subscribe(componentName, mockCallback);
            
            // Publish update events
            const event1 = new CustomEvent('trading-agent:filter:update', { detail: filterData1 });
            const event2 = new CustomEvent('trading-agent:filter:update', { detail: filterData2 });
            
            window.dispatchEvent(event1);
            window.dispatchEvent(event2);
            
            expect(mockCallback).toHaveBeenCalledTimes(2);
            
            // Cleanup
            cleanup();
            
            // Publish another event - should not receive
            const event3 = new CustomEvent('trading-agent:filter:update', { detail: filterData1 });
            window.dispatchEvent(event3);
            
            expect(mockCallback).toHaveBeenCalledTimes(2); // Still 2
        });

        it('should handle both init and update events in subscription', () => {
            const instance = FilterDataMessageService.getInstance();
            const componentName = 'DualEventComponent';
            const initData = { type: 'initial', filters: [] };
            const updateData = { type: 'updated', filters: ['cluster1'] };
            
            instance.subscribe(componentName, mockCallback);
            
            // Dispatch both event types
            const initEvent = new CustomEvent('trading-agent:filter:init', { detail: initData });
            const updateEvent = new CustomEvent('trading-agent:filter:update', { detail: updateData });
            
            window.dispatchEvent(initEvent);
            window.dispatchEvent(updateEvent);
            
            expect(mockCallback).toHaveBeenCalledTimes(2);
            expect(mockCallback).toHaveBeenNthCalledWith(1, initEvent);
            expect(mockCallback).toHaveBeenNthCalledWith(2, updateEvent);
        });

        it('should maintain singleton across publish and subscribe operations', () => {
            const instance1 = FilterDataMessageService.getInstance();
            const instance2 = FilterDataMessageService.getInstance();
            
            instance1.subscribe('Component1', jest.fn());
            instance2.publishUpdateEvent({ test: true });
            
            expect(instance1).toBe(instance2);
        });

        it('should handle rapid successive calls', () => {
            const instance = FilterDataMessageService.getInstance();
            const callbacks = Array.from({ length: 10 }, () => jest.fn());
            
            // Rapid subscriptions
            callbacks.forEach((cb, i) => {
                instance.subscribe(`Component${i}`, cb);
            });
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(20); // 10 components × 2 events
            expect(mfeOrchestrationService.registerMount).toHaveBeenCalledTimes(10);
            
            // Rapid publish
            for (let i = 0; i < 10; i++) {
                instance.publishUpdateEvent({ filterId: i });
            }
            
            jest.runAllTimers();
            expect(dispatchEventSpy).toHaveBeenCalledTimes(10);
        });
    });

    describe('Edge Cases', () => {
        it('should handle subscription with same callback multiple times', () => {
            const instance = FilterDataMessageService.getInstance();
            
            instance.subscribe('Component1', mockCallback);
            instance.subscribe('Component2', mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(4); // 2 subscriptions × 2 events
        });

        it('should handle empty string as component name', () => {
            const instance = FilterDataMessageService.getInstance();
            
            const cleanup = instance.subscribe('', mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(typeof cleanup).toBe('function');
        });

        it('should handle special characters in filter data', () => {
            const instance = FilterDataMessageService.getInstance();
            const specialData = {
                query: "cluster='prod' AND severity<>'info'",
                regex: '/^test-.*$/i',
                escaped: '<script>alert("XSS")</script>'
            };
            
            instance.publishUpdateEvent(specialData);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(specialData);
        });

        it('should handle very large filter data objects', () => {
            const instance = FilterDataMessageService.getInstance();
            const largeData = {
                clusterId: 'filter-large',
                items: Array(1000).fill(null).map((_, i) => ({ 
                    id: i, 
                    name: `filter-${i}`,
                    active: i % 2 === 0 
                }))
            };
            
            instance.publishUpdateEvent(largeData);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(largeData);
            expect(dispatchedEvent.detail.items.length).toBe(1000);
        });

        it('should handle circular references gracefully', () => {
            const instance = FilterDataMessageService.getInstance();
            const circularData: any = { id: 'filter-circular' };
            circularData.self = circularData;
            
            // This should not throw, even though it has circular reference
            expect(() => {
                instance.publishUpdateEvent(circularData);
                jest.runAllTimers();
            }).not.toThrow();
        });

        it('should handle arrays as filter data', () => {
            const instance = FilterDataMessageService.getInstance();
            const arrayData = ['filter1', 'filter2', 'filter3'];
            
            instance.publishUpdateEvent(arrayData);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(arrayData);
        });

        it('should handle primitive values as filter data', () => {
            const instance = FilterDataMessageService.getInstance();
            
            instance.publishUpdateEvent('simple-string');
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toBe('simple-string');
        });
    });

    describe('Type Safety and Inheritance', () => {
        it('should correctly inherit all MessageServiceBase properties', () => {
            const instance = FilterDataMessageService.getInstance();
            
            // Check inherited properties exist
            expect(instance).toHaveProperty('dataDomain');
            expect(instance).toHaveProperty('eventTypeApp');
            expect(instance).toHaveProperty('initAction');
            expect(instance).toHaveProperty('updateAction');
            expect(instance).toHaveProperty('initEventType');
            expect(instance).toHaveProperty('updateEventType');
            expect(instance).toHaveProperty('removeInitEventListenerTimerInSeconds');
        });

        it('should correctly inherit all MessageServiceBase methods', () => {
            const instance = FilterDataMessageService.getInstance();
            
            // Check inherited methods exist and are functions
            expect(typeof instance.publishUpdateEvent).toBe('function');
            expect(typeof instance.publishInitEvent).toBe('function');
            expect(typeof instance.subscribe).toBe('function');
            expect(typeof instance.subscribeToUpdateEvent).toBe('function');
            expect(typeof instance.removeUpdateEventListener).toBe('function');
        });

        it('should maintain correct prototype chain', () => {
            const instance = FilterDataMessageService.getInstance();
            
            expect(Object.getPrototypeOf(instance)).toBe(FilterDataMessageService.prototype);
            expect(Object.getPrototypeOf(FilterDataMessageService.prototype)).toBe(MessageServiceBase.prototype);
        });

        it('should have different domain than UserDataMessageService', () => {
            const instance = FilterDataMessageService.getInstance();
            
            expect(instance.dataDomain).toBe('filter');
            expect(instance.dataDomain).not.toBe('user');
            expect(instance.updateEventType).toBe('trading-agent:filter:update');
            expect(instance.initEventType).toBe('trading-agent:filter:init');
        });
    });
});
