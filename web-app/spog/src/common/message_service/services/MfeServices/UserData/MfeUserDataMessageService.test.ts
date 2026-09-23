import { MfeUserDataMessageService } from './MfeUserDataMessageService';
import { UserDataMessageService } from '../../MessageService/UserDataMessageService/UserDataMessageService';
import * as UserDataServiceModule from '../../DomainService/UserDataService';

jest.mock('../../MessageService/UserDataMessageService/UserDataMessageService');
jest.mock('../../DomainService/UserDataService');

// Type for resetting the singleton
type MfeUserDataServiceType = typeof MfeUserDataMessageService & {
    [key: string]: any;
};

describe('MfeUserDataMessageService', () => {
    let mockUserDataMessageService: jest.Mocked<UserDataMessageService>;
    let mockGetUserDataService: jest.MockedFunction<typeof UserDataServiceModule.getUserDataService>;
    let addEventListenerSpy: jest.SpyInstance;
    let removeEventListenerSpy: jest.SpyInstance;

    beforeEach(() => {
        jest.clearAllMocks();

        mockUserDataMessageService = {
            publishInitEvent: jest.fn(),
            publishUpdateEvent: jest.fn(),
            getInstance: jest.fn(),
        } as unknown as jest.Mocked<UserDataMessageService>;

        (UserDataMessageService.getInstance as jest.Mock).mockReturnValue(
            mockUserDataMessageService
        );

        mockGetUserDataService = UserDataServiceModule.getUserDataService as jest.MockedFunction<
            typeof UserDataServiceModule.getUserDataService
        >;

        addEventListenerSpy = jest.spyOn(window, 'addEventListener');
        removeEventListenerSpy = jest.spyOn(window, 'removeEventListener');
    });

    afterEach(() => {
        addEventListenerSpy.mockRestore();
        removeEventListenerSpy.mockRestore();
        // Reset singleton instance using type-safe approach
        const MfeServiceType = MfeUserDataMessageService as MfeUserDataServiceType;
        MfeServiceType['instance'] = null;
    });

    describe('getInstance', () => {
        it('should return the same instance on multiple calls', () => {
            const instance1 = MfeUserDataMessageService.getInstance();
            const instance2 = MfeUserDataMessageService.getInstance();

            expect(instance1).toBe(instance2);
        });

        it('should create a new instance on first call', () => {
            const instance = MfeUserDataMessageService.getInstance();

            expect(instance).toBeInstanceOf(MfeUserDataMessageService);
        });
    });

    describe('subscribe', () => {
        it('should add event listeners for both update and init events', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:update', handler);
            expect(addEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:init', handler);
            expect(addEventListenerSpy).toHaveBeenCalledTimes(2);
        });

        it('should return an unsubscribe function', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            const unsubscribe = instance.subscribe('TestComponent', handler);

            expect(typeof unsubscribe).toBe('function');
        });

        it('should remove event listeners when unsubscribe is called', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            const unsubscribe = instance.subscribe('TestComponent', handler);
            unsubscribe();

            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:update', handler);
            expect(removeEventListenerSpy).toHaveBeenCalledWith('trading-agent:user:init', handler);
        });

        it('should call publishInitEvent when current state exists', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            const mockState = { userId: '123', userName: 'Test User' };
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(mockState),
            } as any);

            instance.subscribe('TestComponent', handler);

            expect(mockUserDataMessageService.publishInitEvent).toHaveBeenCalledWith(mockState);
        });

        it('should not call publishInitEvent when current state is null', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            expect(mockUserDataMessageService.publishInitEvent).not.toHaveBeenCalled();
        });

        it('should pass componentName parameter', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            const componentName = 'MyComponent';
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe(componentName, handler);

            // Verify the method was called (componentName is stored in the call)
            expect(addEventListenerSpy).toHaveBeenCalled();
        });
    });

    describe('publish', () => {
        it('should call publishUpdateEvent with provided data', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const testData = { userId: '456', userName: 'Another User' };

            instance.publish(testData);

            expect(mockUserDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(testData);
        });

        it('should handle any data type', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const testString = 'test data';

            instance.publish(testString);

            expect(mockUserDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(testString);
        });

        it('should call publishUpdateEvent multiple times', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const data1 = { id: '1' };
            const data2 = { id: '2' };

            instance.publish(data1);
            instance.publish(data2);

            expect(mockUserDataMessageService.publishUpdateEvent).toHaveBeenCalledTimes(2);
            expect(mockUserDataMessageService.publishUpdateEvent).toHaveBeenNthCalledWith(1, data1);
            expect(mockUserDataMessageService.publishUpdateEvent).toHaveBeenNthCalledWith(2, data2);
        });
    });

    describe('Event Types', () => {
        it('should use correct update event type', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            const updateEventCall = addEventListenerSpy.mock.calls.find(
                (call) => call[0] === 'trading-agent:user:update'
            );
            expect(updateEventCall).toBeDefined();
        });

        it('should use correct init event type', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('TestComponent', handler);

            const initEventCall = addEventListenerSpy.mock.calls.find(
                (call) => call[0] === 'trading-agent:user:init'
            );
            expect(initEventCall).toBeDefined();
        });
    });

    describe('Integration', () => {
        it('should handle subscribe and publish in sequence', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler = jest.fn();
            const initialState = { userId: '123' };
            const updateData = { userId: '123', updated: true };

            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(initialState),
            } as any);

            instance.subscribe('TestComponent', handler);
            instance.publish(updateData);

            expect(mockUserDataMessageService.publishInitEvent).toHaveBeenCalledWith(initialState);
            expect(mockUserDataMessageService.publishUpdateEvent).toHaveBeenCalledWith(updateData);
        });

        it('should allow multiple subscriptions', () => {
            const instance = MfeUserDataMessageService.getInstance();
            const handler1 = jest.fn();
            const handler2 = jest.fn();

            mockGetUserDataService.mockReturnValue({
                getState: jest.fn().mockReturnValue(null),
            } as any);

            instance.subscribe('Component1', handler1);
            instance.subscribe('Component2', handler2);

            expect(addEventListenerSpy).toHaveBeenCalledTimes(4);
        });
    });
});