/**
 * Tests for UserDataMessageService
 * Testing singleton pattern, inheritance from MessageServiceBase, and user domain specific functionality
 */
import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { UserDataMessageService } from './UserDataMessageService';
import MessageServiceBase from '../MessageServiceBase';

// Mock the MfeOrchestrationService
jest.mock('../../MfeOrchestrationService', () => ({
    __esModule: true,
    default: {
        registerMount: jest.fn(),
    },
}));

import mfeOrchestrationService from '../../MfeOrchestrationService';

describe('UserDataMessageService', () => {
    let dispatchEventSpy: jest.SpyInstance;
    let addEventListenerSpy: jest.SpyInstance;
    let removeEventListenerSpy: jest.SpyInstance;
    let mockCallback: jest.Mock;

    beforeEach(() => {
        // Reset the singleton instance before each test
        (UserDataMessageService as any).instance = null;
        
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
        (UserDataMessageService as any).instance = null;
    });

    describe('Singleton Pattern', () => {
        it('should create a singleton instance', () => {
            const instance1 = UserDataMessageService.getInstance();
            const instance2 = UserDataMessageService.getInstance();
            
            expect(instance1).toBe(instance2);
            expect(instance1).toBeInstanceOf(UserDataMessageService);
        });

        it('should only create one instance across multiple calls', () => {
            const instances = Array.from({ length: 5 }, () => UserDataMessageService.getInstance());
            
            const firstInstance = instances[0];
            instances.forEach(instance => {
                expect(instance).toBe(firstInstance);
            });
        });

        it('should be an instance of MessageServiceBase', () => {
            const instance = UserDataMessageService.getInstance();
            expect(instance).toBeInstanceOf(MessageServiceBase);
        });

        it('should not allow direct instantiation', () => {
            // TypeScript will prevent this at compile time, but we can verify the constructor is private
            expect(() => {
                // @ts-ignore - Testing private constructor
                new UserDataMessageService();
            }).not.toThrow();
            
            // But getInstance should still return the same instance
            const instance1 = UserDataMessageService.getInstance();
            // @ts-ignore - Testing private constructor
            const instance2 = new UserDataMessageService();
            expect(instance2).toBeInstanceOf(UserDataMessageService);
        });
    });

    describe('Initialization', () => {
        it('should initialize with correct data domain', () => {
            const instance = UserDataMessageService.getInstance();
            expect(instance.dataDomain).toBe('user');
        });

        it('should initialize with correct event types', () => {
            const instance = UserDataMessageService.getInstance();
            
            expect(instance.eventTypeApp).toBe('trading-agent');
            expect(instance.initAction).toBe('init');
            expect(instance.updateAction).toBe('update');
        });

        it('should create correct update event type for user domain', () => {
            const instance = UserDataMessageService.getInstance();
            expect(instance.updateEventType).toBe('trading-agent:user:update');
        });

        it('should create correct init event type for user domain', () => {
            const instance = UserDataMessageService.getInstance();
            expect(instance.initEventType).toBe('trading-agent:user:init');
        });

        it('should set default timer value', () => {
            const instance = UserDataMessageService.getInstance();
            expect(instance.removeInitEventListenerTimerInSeconds).toBe(2);
        });
    });

    describe('Inherited Methods - publishUpdateEvent', () => {
        it('should publish update event with user data', () => {
            const instance = UserDataMessageService.getInstance();
            const userData = { 
                id: 'user-123', 
                name: 'Test User',
                email: 'test@example.com' 
            };
            
            instance.publishUpdateEvent(userData);
            
            // Fast-forward timers
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.type).toBe('trading-agent:user:update');
            expect(dispatchedEvent.detail).toEqual(userData);
        });

        it('should publish update event asynchronously', () => {
            const instance = UserDataMessageService.getInstance();
            const userData = { id: 'user-456' };
            
            instance.publishUpdateEvent(userData);
            
            // Before timer runs
            expect(dispatchEventSpy).not.toHaveBeenCalled();
            
            // After timer runs
            jest.runAllTimers();
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
        });

        it('should handle multiple update events', () => {
            const instance = UserDataMessageService.getInstance();
            const userData1 = { id: 'user-1', name: 'User One' };
            const userData2 = { id: 'user-2', name: 'User Two' };
            const userData3 = { id: 'user-3', name: 'User Three' };
            
            instance.publishUpdateEvent(userData1);
            instance.publishUpdateEvent(userData2);
            instance.publishUpdateEvent(userData3);
            
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(3);
            
            const events = dispatchEventSpy.mock.calls.map(call => call[0] as CustomEvent);
            expect(events[0].detail).toEqual(userData1);
            expect(events[1].detail).toEqual(userData2);
            expect(events[2].detail).toEqual(userData3);
        });

        it('should handle null detail', () => {
            const instance = UserDataMessageService.getInstance();
            
            instance.publishUpdateEvent(null);
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toBeNull();
        });

        it('should handle undefined detail', () => {
            const instance = UserDataMessageService.getInstance();
            
            instance.publishUpdateEvent(undefined);
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            // CustomEvent converts undefined to null
            expect(dispatchedEvent.detail).toBeNull();
        });

        it('should handle complex nested user data objects', () => {
            const instance = UserDataMessageService.getInstance();
            const complexUserData = {
                id: 'user-999',
                profile: {
                    name: 'Complex User',
                    preferences: {
                        theme: 'dark',
                        notifications: true
                    }
                },
                roles: ['admin', 'user'],
                metadata: {
                    lastLogin: new Date().toISOString(),
                    settings: { key: 'value' }
                }
            };
            
            instance.publishUpdateEvent(complexUserData);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(complexUserData);
        });
    });

    describe('Inherited Methods - publishInitEvent', () => {
        it('should publish init event with user data', () => {
            const instance = UserDataMessageService.getInstance();
            const userData = { id: 'user-initial', name: 'Initial User' };
            
            instance.publishInitEvent(userData);
            jest.runAllTimers();
            
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.type).toBe('trading-agent:user:init');
            expect(dispatchedEvent.detail).toEqual(userData);
        });

        it('should publish init event asynchronously', () => {
            const instance = UserDataMessageService.getInstance();
            const userData = { id: 'user-init' };
            
            instance.publishInitEvent(userData);
            
            // Before timer runs
            expect(dispatchEventSpy).not.toHaveBeenCalled();
            
            // After timer runs
            jest.runAllTimers();
            expect(dispatchEventSpy).toHaveBeenCalledTimes(1);
        });

        it('should handle empty object as detail', () => {
            const instance = UserDataMessageService.getInstance();
            
            instance.publishInitEvent({});
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual({});
        });
    });

    describe('Inherited Methods - subscribe', () => {
        it('should subscribe to both update and init events', () => {
            const instance = UserDataMessageService.getInstance();
            const componentName = 'TestComponent';
            
            instance.subscribe(componentName, mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:update', mockCallback);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:init', mockCallback);
        });

        it('should register mount with orchestration service', () => {
            const instance = UserDataMessageService.getInstance();
            const componentName = 'TestComponent';
            
            instance.subscribe(componentName, mockCallback);
            
            expect(mfeOrchestrationService.registerMount).toHaveBeenCalledTimes(1);
        });

        it('should return cleanup function', () => {
            const instance = UserDataMessageService.getInstance();
            const componentName = 'TestComponent';
            
            const cleanup = instance.subscribe(componentName, mockCallback);
            
            expect(typeof cleanup).toBe('function');
        });

        it('should cleanup both event listeners when cleanup function is called', () => {
            const instance = UserDataMessageService.getInstance();
            const componentName = 'TestComponent';
            
            const cleanup = instance.subscribe(componentName, mockCallback);
            
            // Clear mocks to check cleanup calls
            jest.clearAllMocks();
            
            cleanup();
            
            expect(removeEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:update', mockCallback);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:init', mockCallback);
        });

        it('should handle multiple subscriptions', () => {
            const instance = UserDataMessageService.getInstance();
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
            const instance = UserDataMessageService.getInstance();
            
            instance.subscribeToUpdateEvent(mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(1);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:update', mockCallback);
        });

        it('should receive update events after subscription', () => {
            const instance = UserDataMessageService.getInstance();
            const userData = { id: 'user-update' };
            
            // Actually add the event listener (not just spy)
            window.addEventListener('trading-agent:user:update', mockCallback);
            
            // Publish event
            instance.publishUpdateEvent(userData);
            jest.runAllTimers();
            
            expect(mockCallback).toHaveBeenCalledTimes(1);
            expect(mockCallback.mock.calls[0][0].detail).toEqual(userData);
            
            // Cleanup
            window.removeEventListener('trading-agent:user:update', mockCallback);
        });
    });

    describe('Inherited Methods - removeUpdateEventListener', () => {
        it('should remove update event listener', () => {
            const instance = UserDataMessageService.getInstance();
            
            instance.removeUpdateEventListener(mockCallback);
            
            expect(removeEventListenerSpy).toHaveBeenCalledTimes(1);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:update', mockCallback);
        });

        it('should stop receiving update events after removal', () => {
            const instance = UserDataMessageService.getInstance();
            const userData = { id: 'user-removed' };
            
            // Subscribe
            instance.subscribeToUpdateEvent(mockCallback);
            
            // Publish and receive event
            const updateEvent1 = new CustomEvent('trading-agent:user:update', { detail: userData });
            window.dispatchEvent(updateEvent1);
            expect(mockCallback).toHaveBeenCalledTimes(1);
            
            // Remove listener
            instance.removeUpdateEventListener(mockCallback);
            
            // Publish again - should not receive
            const updateEvent2 = new CustomEvent('trading-agent:user:update', { detail: userData });
            window.dispatchEvent(updateEvent2);
            expect(mockCallback).toHaveBeenCalledTimes(1); // Still 1, not 2
        });
    });

    describe('Integration Tests', () => {
        it('should handle complete lifecycle: subscribe, receive updates, cleanup', () => {
            const instance = UserDataMessageService.getInstance();
            const componentName = 'LifecycleComponent';
            const userData1 = { id: 'user-1', action: 'login' };
            const userData2 = { id: 'user-1', action: 'update-profile' };
            
            // Subscribe
            const cleanup = instance.subscribe(componentName, mockCallback);
            
            // Publish update events
            const event1 = new CustomEvent('trading-agent:user:update', { detail: userData1 });
            const event2 = new CustomEvent('trading-agent:user:update', { detail: userData2 });
            
            window.dispatchEvent(event1);
            window.dispatchEvent(event2);
            
            expect(mockCallback).toHaveBeenCalledTimes(2);
            
            // Cleanup
            cleanup();
            
            // Publish another event - should not receive
            const event3 = new CustomEvent('trading-agent:user:update', { detail: userData1 });
            window.dispatchEvent(event3);
            
            expect(mockCallback).toHaveBeenCalledTimes(2); // Still 2
        });

        it('should handle both init and update events in subscription', () => {
            const instance = UserDataMessageService.getInstance();
            const componentName = 'DualEventComponent';
            const initData = { id: 'user-init', type: 'initial' };
            const updateData = { id: 'user-update', type: 'updated' };
            
            instance.subscribe(componentName, mockCallback);
            
            // Dispatch both event types
            const initEvent = new CustomEvent('trading-agent:user:init', { detail: initData });
            const updateEvent = new CustomEvent('trading-agent:user:update', { detail: updateData });
            
            window.dispatchEvent(initEvent);
            window.dispatchEvent(updateEvent);
            
            expect(mockCallback).toHaveBeenCalledTimes(2);
            expect(mockCallback).toHaveBeenNthCalledWith(1, initEvent);
            expect(mockCallback).toHaveBeenNthCalledWith(2, updateEvent);
        });

        it('should maintain singleton across publish and subscribe operations', () => {
            const instance1 = UserDataMessageService.getInstance();
            const instance2 = UserDataMessageService.getInstance();
            
            instance1.subscribe('Component1', jest.fn());
            instance2.publishUpdateEvent({ id: 'user-test' });
            
            expect(instance1).toBe(instance2);
        });

        it('should handle rapid successive calls', () => {
            const instance = UserDataMessageService.getInstance();
            const callbacks = Array.from({ length: 10 }, () => jest.fn());
            
            // Rapid subscriptions
            callbacks.forEach((cb, i) => {
                instance.subscribe(`Component${i}`, cb);
            });
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(20); // 10 components × 2 events
            expect(mfeOrchestrationService.registerMount).toHaveBeenCalledTimes(10);
            
            // Rapid publish
            for (let i = 0; i < 10; i++) {
                instance.publishUpdateEvent({ id: `user-${i}` });
            }
            
            jest.runAllTimers();
            expect(dispatchEventSpy).toHaveBeenCalledTimes(10);
        });
    });

    describe('Edge Cases', () => {
        it('should handle subscription with same callback multiple times', () => {
            const instance = UserDataMessageService.getInstance();
            
            instance.subscribe('Component1', mockCallback);
            instance.subscribe('Component2', mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(4); // 2 subscriptions × 2 events
        });

        it('should handle empty string as component name', () => {
            const instance = UserDataMessageService.getInstance();
            
            const cleanup = instance.subscribe('', mockCallback);
            
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
            expect(typeof cleanup).toBe('function');
        });

        it('should handle special characters in user data', () => {
            const instance = UserDataMessageService.getInstance();
            const specialData = {
                name: "O'Brien <test@test.com>",
                description: 'User with "quotes" and \'apostrophes\'',
                script: '<script>alert("XSS")</script>'
            };
            
            instance.publishUpdateEvent(specialData);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(specialData);
        });

        it('should handle very large user data objects', () => {
            const instance = UserDataMessageService.getInstance();
            const largeData = {
                id: 'user-large',
                data: Array(1000).fill(null).map((_, i) => ({ id: i, value: `item-${i}` }))
            };
            
            instance.publishUpdateEvent(largeData);
            jest.runAllTimers();
            
            const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent;
            expect(dispatchedEvent.detail).toEqual(largeData);
            expect(dispatchedEvent.detail.data.length).toBe(1000);
        });

        it('should handle circular references gracefully', () => {
            const instance = UserDataMessageService.getInstance();
            const circularData: any = { id: 'user-circular' };
            circularData.self = circularData;
            
            // This should not throw, even though it has circular reference
            expect(() => {
                instance.publishUpdateEvent(circularData);
                jest.runAllTimers();
            }).not.toThrow();
        });
    });

    describe('Type Safety and Inheritance', () => {
        it('should correctly inherit all MessageServiceBase properties', () => {
            const instance = UserDataMessageService.getInstance();
            
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
            const instance = UserDataMessageService.getInstance();
            
            // Check inherited methods exist and are functions
            expect(typeof instance.publishUpdateEvent).toBe('function');
            expect(typeof instance.publishInitEvent).toBe('function');
            expect(typeof instance.subscribe).toBe('function');
            expect(typeof instance.subscribeToUpdateEvent).toBe('function');
            expect(typeof instance.removeUpdateEventListener).toBe('function');
        });

        it('should maintain correct prototype chain', () => {
            const instance = UserDataMessageService.getInstance();
            
            expect(Object.getPrototypeOf(instance)).toBe(UserDataMessageService.prototype);
            expect(Object.getPrototypeOf(UserDataMessageService.prototype)).toBe(MessageServiceBase.prototype);
        });
    });
});
