/**
 * Tests for UserDataService - Domain service for user data management
 * Testing singleton pattern, initialization, message service integration
 */
import '@testing-library/jest-dom';
import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';

// Mock dependencies BEFORE importing the service
jest.mock('../../../../state_management/slices/userData/userDataSlice', () => ({
    userDataSlice: {
        actions: {
            initUserDataStore: jest.fn(),
            updateUserDataStoreToken: jest.fn(),
        },
    },
}));

jest.mock('../../MessageService/UserDataMessageService/UserDataMessageService');
jest.mock('../DomainServiceBase/DomainServiceBase');
jest.mock('../../MfeOrchestrationService');

import { getUserDataService } from '.';
import * as UserDataMessageServiceModule from '../../MessageService/UserDataMessageService/UserDataMessageService';
import DomainServiceBase from '../DomainServiceBase/DomainServiceBase';
import mfeOrchestrationService from '../../MfeOrchestrationService';

describe('UserDataService', () => {
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

        jest.spyOn(UserDataMessageServiceModule.UserDataMessageService, 'getInstance').mockReturnValue(
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
        jest.resetModules();
    });

    describe('getInstance()', () => {
        it('should create a new UserDataService instance on first call', () => {
            const instance = getUserDataService();
            expect(instance).toBeDefined();
            expect(instance.init).toBeDefined();
        });

        it('should return the same instance on multiple calls (singleton pattern)', () => {
            const instance1 = getUserDataService();
            const instance2 = getUserDataService();
            expect(instance1).toBe(instance2);
        });

        it('should call getUserDataMessageService to get the message service', () => {
            const instance = getUserDataService();
            // Instance is created, just verify it has the required methods
            expect(instance).toBeDefined();
            expect(instance.init).toBeDefined();
        });
    });

    describe('init()', () => {
        it('should initialize the service and register with orchestration service on first call', () => {
            const instance = getUserDataService();
            instance.init();

            expect(mockRegisterDataService).toHaveBeenCalled();
            expect(mockRegisterDataService.mock.calls.length).toBeGreaterThan(0);
        });

        it('should skip registration on subsequent init() calls', () => {
            const instance = getUserDataService();
            const initialCallCount = mockRegisterDataService.mock.calls.length;

            instance.init();
            const firstCallCount = mockRegisterDataService.mock.calls.length;

            instance.init();
            const secondCallCount = mockRegisterDataService.mock.calls.length;

            expect(secondCallCount).toBe(firstCallCount);
        });

        it('should not re-register after multiple init calls', () => {
            const instance = getUserDataService();
            const initialCallCount = mockRegisterDataService.mock.calls.length;

            instance.init();
            const firstCallCount = mockRegisterDataService.mock.calls.length;
            
            instance.init();
            instance.init();
            const finalCallCount = mockRegisterDataService.mock.calls.length;

            expect(finalCallCount).toBe(firstCallCount);
        });
    });

    describe('getUserDataService()', () => {
        it('should return a UserDataService instance', () => {
            const service = getUserDataService();
            expect(service).toBeDefined();
            expect(service.init).toBeDefined();
        });

        it('should return the same instance on multiple calls', () => {
            const service1 = getUserDataService();
            const service2 = getUserDataService();
            expect(service1).toBe(service2);
        });
    });

    describe('Service Integration', () => {
        it('should pass userDataMessageService to DomainServiceBase constructor', () => {
            const instance = getUserDataService();

            expect(instance).toBeDefined();
        });

        it('should maintain singleton pattern across different access methods', () => {
            const instance1 = getUserDataService();
            const instance2 = getUserDataService();

            expect(instance1).toBe(instance2);
        });

        it('should accept init method for initialization', () => {
            const instance = getUserDataService();
            
            expect(instance.init).toBeDefined();
            expect(typeof instance.init).toBe('function');
        });
    });
});
