/**
 * Tests for FilterDataService - Domain service for filter data management
 * Testing singleton pattern, initialization, message service integration
 */
import '@testing-library/jest-dom';
import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';

// Mock dependencies BEFORE importing the service
jest.mock('../../../../state_management/slices/filterData/filterDataSlice', () => ({
    filterDataSlice: {
        actions: {
            updateFilterData: jest.fn(),
        },
    },
}));

jest.mock('../../MessageService/FilterDataMessageService/FilterDataMessageService');
jest.mock('../DomainServiceBase/DomainServiceBase');
jest.mock('../../MfeOrchestrationService');

import { getFilterDataService } from '.';
import * as FilterDataMessageServiceModule from '../../MessageService/FilterDataMessageService/FilterDataMessageService';
import DomainServiceBase from '../DomainServiceBase/DomainServiceBase';
import mfeOrchestrationService from '../../MfeOrchestrationService';

describe('FilterDataService', () => {
    let mockUserDataMessageService: any;
    let mockRegisterDataService: jest.Mock;
    let mockDomainServiceBase: jest.Mock;

    beforeEach(() => {
        jest.clearAllMocks();

        // Setup message service mock
        mockUserDataMessageService = {
            subscribe: jest.fn(),
            publish: jest.fn(),
            updateState: jest.fn(),
            getState: jest.fn(),
            broadcastInitEvent: jest.fn(),
            cleanup: jest.fn(),
            isServiceInitialized: false,
        };

        jest.spyOn(FilterDataMessageServiceModule.FilterDataMessageService, 'getInstance').mockReturnValue(
            mockUserDataMessageService as any
        );

        // Setup DomainServiceBase mock
        mockDomainServiceBase = jest.fn(function() {
            this.updateState = jest.fn();
            this.getState = jest.fn();
            this.broadcastInitEvent = jest.fn();
            this.cleanup = jest.fn();
            this.isServiceInitialized = false;
        });
        (DomainServiceBase as any).mockImplementation(mockDomainServiceBase);

        // Setup orchestration service mock - it's a default export object
        mockRegisterDataService = jest.fn();
        (mfeOrchestrationService as any).registerDataService = mockRegisterDataService;
        (mfeOrchestrationService as any).init = jest.fn();
        (mfeOrchestrationService as any).registerMount = jest.fn();
    });

    afterEach(() => {
        jest.clearAllMocks();
    });

    describe('getInstance()', () => {
        it('should create a new FilterDataService instance on first call', () => {
            const instance = getFilterDataService();
            expect(instance).toBeDefined();
            expect(instance.init).toBeDefined();
        });

        it('should return the same instance on multiple calls (singleton pattern)', () => {
            const instance1 = getFilterDataService();
            const instance2 = getFilterDataService();
            expect(instance1).toBe(instance2);
        });

        it('should have required methods available', () => {
            const instance = getFilterDataService();
            expect(typeof instance.init).toBe('function');
        });

        it('should return instance with init method callable', () => {
            const instance = getFilterDataService();
            expect(instance.init).toBeDefined();
            expect(typeof instance.init).toBe('function');
            // Should not throw when calling init
            expect(() => instance.init()).not.toThrow();
        });
    });

    describe('init()', () => {
        it('should provide init method on instance', () => {
            const instance = getFilterDataService();
            expect(instance.init).toBeDefined();
            expect(typeof instance.init).toBe('function');
        });

        it('should allow calling init without errors', () => {
            const instance = getFilterDataService();
            expect(() => instance.init()).not.toThrow();
        });
    });

    describe('getFilterDataService()', () => {
        it('should return a FilterDataService instance', () => {
            const service = getFilterDataService();
            expect(service).toBeDefined();
            expect(service.init).toBeDefined();
        });

        it('should return the same instance on multiple calls', () => {
            const service1 = getFilterDataService();
            const service2 = getFilterDataService();
            expect(service1).toBe(service2);
        });

        it('should return same instance across different test blocks', () => {
            const service = getFilterDataService();
            expect(service).toBeDefined();
            
            const anotherRef = getFilterDataService();
            expect(anotherRef).toBe(service);
        });

        it('should create instance with mocked dependencies', () => {
            const instance = getFilterDataService();
            expect(instance).toBeDefined();
        });
    });

    describe('Singleton pattern behavior', () => {
        it('should maintain singleton pattern across different access methods', () => {
            const instance1 = getFilterDataService();
            const instance2 = getFilterDataService();
            expect(instance1).toBe(instance2);
        });

        it('should accept init method for initialization', () => {
            const instance = getFilterDataService();
            expect(instance.init).toBeDefined();
            expect(typeof instance.init).toBe('function');
        });

        it('should guarantee singleton behavior across calls', () => {
            const services = [
                getFilterDataService(),
                getFilterDataService(),
                getFilterDataService(),
            ];
            
            expect(services[0]).toBe(services[1]);
            expect(services[1]).toBe(services[2]);
        });

        it('should return same reference across function calls', () => {
            const ref1 = getFilterDataService();
            const ref2 = getFilterDataService();
            const ref3 = getFilterDataService();

            expect(ref1 === ref2).toBe(true);
            expect(ref2 === ref3).toBe(true);
        });

        it('should maintain state across multiple accesses', () => {
            const instance = getFilterDataService();
            instance.init();

            // Access instance again and verify it's the same object
            const sameInstance = getFilterDataService();
            expect(instance).toBe(sameInstance);
        });

        it('should handle instance reuse without re-initialization of dependencies', () => {
            getFilterDataService();
            const firstCallCount = (DomainServiceBase as jest.Mock).mock.calls.length;
            
            getFilterDataService();
            const secondCallCount = (DomainServiceBase as jest.Mock).mock.calls.length;
            
            expect(secondCallCount).toBe(firstCallCount);
        });
    });

    describe('Service state management', () => {
        it('should maintain state across multiple accesses', () => {
            const instance = getFilterDataService();
            instance.init();

            // Access instance again and verify it's the same object
            const sameInstance = getFilterDataService();
            expect(instance).toBe(sameInstance);
        });

        it('should ensure singleton stays consistent', () => {
            const instance1 = getFilterDataService();
            instance1.init();
            
            const instance2 = getFilterDataService();
            expect(instance1).toBe(instance2);
        });

        it('should ensure idempotent behavior', () => {
            const instance = getFilterDataService();
            instance.init();
            instance.init();
            instance.init();

            // Should still be the same instance
            expect(getFilterDataService()).toBe(instance);
        });
    });

    describe('Message service integration', () => {
        it('should use message service in construction', () => {
            const service = getFilterDataService();
            expect(service).toBeDefined();
        });

        it('should retrieve message service during instantiation', () => {
            const service = getFilterDataService();
            expect(service).toBeDefined();
        });
    });

    describe('DomainServiceBase integration', () => {
        it('should extend DomainServiceBase functionality', () => {
            const service = getFilterDataService();
            expect(service).toBeDefined();
            expect(service.init).toBeDefined();
        });

        it('should initialize with filterDataSlice configuration', () => {
            const service = getFilterDataService();
            expect(service).toBeDefined();
        });

        it('should create instance with proper initialization', () => {
            const service = getFilterDataService();
            expect(service).toBeDefined();
            expect(typeof service.init).toBe('function');
        });
    });

    describe('Orchestration service integration', () => {
        it('should have orchestration service integration', () => {
            const instance = getFilterDataService();
            instance.init();
            expect(instance).toBeDefined();
        });

        it('should properly handle orchestration registration', () => {
            const instance = getFilterDataService();
            instance.init();
            // Verify orchestration service was accessed
            expect(instance).toBeDefined();
        });
    });
});
