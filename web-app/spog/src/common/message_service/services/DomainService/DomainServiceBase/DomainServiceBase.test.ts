import DomainServiceBase from './DomainServiceBase';
import MessageServiceBase from '../../MessageService/MessageServiceBase';
import store from '../../../../state_management/store/dataStore';


jest.mock('../../../../state_management/store/dataStore');

describe('DomainServiceBase', () => {
    let mockMessageService: jest.Mocked<MessageServiceBase>;
    let mockSliceIntf: any;
    let unsubscribeMock: jest.Mock;
    let storSubscribeMock: jest.Mock;
    let storeDispatchMock: jest.Mock;
    let storeGetStateMock: jest.Mock;

    beforeEach(() => {
        jest.clearAllMocks();

        // Mock store functions
        unsubscribeMock = jest.fn();
        storSubscribeMock = jest.fn(() => unsubscribeMock);
        storeDispatchMock = jest.fn();
        storeGetStateMock = jest.fn(() => ({
            userData: {
                id: 'user-1',
                name: 'Test User',
            },
        }));

        (store.subscribe as jest.Mock) = storSubscribeMock;
        (store.dispatch as jest.Mock) = storeDispatchMock;
        (store.getState as jest.Mock) = storeGetStateMock;

        // Mock MessageServiceBase
        mockMessageService = {
            subscribeToUpdateEvent: jest.fn(),
            publishUpdateEvent: jest.fn(),
            publishInitEvent: jest.fn(),
        } as any;

        // Mock SliceIntf
        mockSliceIntf = {
            name: 'userData',
            update: jest.fn((data) => ({ type: 'UPDATE_USER_DATA', payload: data })),
        };
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    describe('constructor', () => {
        it('should initialize properties correctly', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect(service.messageService).toBe(mockMessageService);
            expect(service.sliceIntf).toBe(mockSliceIntf);
            expect(service.slice).toBe('userData');
        });

        it('should subscribe to message service update events', () => {
            new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect(mockMessageService.subscribeToUpdateEvent).toHaveBeenCalledWith(
                expect.any(Function)
            );
        });

        it('should call subscribe method during initialization', () => {
            const subscribeSpy = jest.spyOn(DomainServiceBase.prototype, 'subscribe');

            new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect(subscribeSpy).toHaveBeenCalled();
            subscribeSpy.mockRestore();
        });

        it('should initialize previousState to current state', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect((service as any).previousState).toEqual({
                id: 'user-1',
                name: 'Test User',
            });
        });

        it('should initialize isPublishing to false', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect((service as any).isPublishing).toBe(false);
        });
    });

    describe('updateHandler', () => {
        it('should update state when CustomEvent is received and not publishing', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const updateStateSpy = jest.spyOn(service, 'updateState');
            const eventDetail = { id: 'user-2', name: 'Updated User' };

            const customEvent = new CustomEvent('update', { detail: eventDetail });
            const handler = mockMessageService.subscribeToUpdateEvent.mock.calls[0][0];

            handler(customEvent);

            expect(updateStateSpy).toHaveBeenCalledWith(eventDetail);
            updateStateSpy.mockRestore();
        });

        it('should ignore events when isPublishing is true', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            (service as any).isPublishing = true;
            const updateStateSpy = jest.spyOn(service, 'updateState');

            const customEvent = new CustomEvent('update', { detail: { id: 'user-2' } });
            const handler = mockMessageService.subscribeToUpdateEvent.mock.calls[0][0];

            handler(customEvent);

            expect(updateStateSpy).not.toHaveBeenCalled();
            updateStateSpy.mockRestore();
        });

        it('should extract detail from CustomEvent correctly', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const updateStateSpy = jest.spyOn(service, 'updateState');
            const complexDetail = {
                id: 'user-3',
                profile: { email: 'test@example.com', role: 'admin' },
                metadata: { createdAt: '2024-01-01' },
            };

            const customEvent = new CustomEvent('update', { detail: complexDetail });
            const handler = mockMessageService.subscribeToUpdateEvent.mock.calls[0][0];

            handler(customEvent);

            expect(updateStateSpy).toHaveBeenCalledWith(complexDetail);
            updateStateSpy.mockRestore();
        });
    });

    describe('subscribe', () => {
        it('should subscribe to Redux store changes', () => {
            new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect(storSubscribeMock).toHaveBeenCalled();
        });

        it('should store initial state in previousState', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect((service as any).previousState).toEqual({
                id: 'user-1',
                name: 'Test User',
            });
        });

        it('should store unsubscribe function', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            expect((service as any).unsubscribeFromStore).toBe(unsubscribeMock);
        });

        it('should publish update event when state changes', () => {
            new DomainServiceBase(mockMessageService, mockSliceIntf);

            const storeSubscriber = storSubscribeMock.mock.calls[0][0];
            storeGetStateMock.mockReturnValue({
                userData: {
                    id: 'user-1-updated',
                    name: 'Updated User',
                },
            });

            storeSubscriber();

            expect(mockMessageService.publishUpdateEvent).toHaveBeenCalledWith({
                id: 'user-1-updated',
                name: 'Updated User',
            });
        });

        it('should not publish update event when state remains the same', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            mockMessageService.publishUpdateEvent.mockClear();

            const storeSubscriber = storSubscribeMock.mock.calls[0][0];
            storeSubscriber();

            expect(mockMessageService.publishUpdateEvent).not.toHaveBeenCalled();
        });

        it('should set isPublishing flag before publishing and reset after', (done) => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const storeSubscriber = storSubscribeMock.mock.calls[0][0];

            expect((service as any).isPublishing).toBe(false);

            storeGetStateMock.mockReturnValue({
                userData: {
                    id: 'user-1-changed',
                    name: 'Changed User',
                },
            });

            storeSubscriber();

            // Flag should be reset asynchronously
            setTimeout(() => {
                expect((service as any).isPublishing).toBe(false);
                done();
            }, 10);
        });

        it('should update previousState after publishing', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const storeSubscriber = storSubscribeMock.mock.calls[0][0];
            const newState = {
                id: 'user-2',
                name: 'New User',
            };

            storeGetStateMock.mockReturnValue({
                userData: newState,
            });

            storeSubscriber();

            expect((service as any).previousState).toEqual(newState);
        });
    });

    describe('cleanup', () => {
        it('should unsubscribe from store when cleanup is called', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            service.cleanup();

            expect(unsubscribeMock).toHaveBeenCalled();
        });

        it('should not throw error if unsubscribeFromStore is null', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            (service as any).unsubscribeFromStore = null;

            expect(() => {
                service.cleanup();
            }).not.toThrow();
        });

        it('should handle cleanup gracefully when called multiple times', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            service.cleanup();
            expect(() => {
                service.cleanup();
            }).not.toThrow();
        });
    });

    describe('getState', () => {
        it('should return the current slice state from Redux store', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            const result = service.getState();

            expect(result).toEqual({
                id: 'user-1',
                name: 'Test User',
            });
        });

        it('should call store.getState', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            storeGetStateMock.mockClear();

            service.getState();

            expect(storeGetStateMock).toHaveBeenCalled();
        });

        it('should return correct slice with different slice names', () => {
            mockSliceIntf.name = 'filterData';
            storeGetStateMock.mockReturnValue({
                filterData: {
                    filters: ['active', 'critical'],
                },
            });

            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            const result = service.getState();

            expect(result).toEqual({
                filters: ['active', 'critical'],
            });
        });

        it('should return undefined if slice does not exist in state', () => {
            storeGetStateMock.mockReturnValue({});

            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            const result = service.getState();

            expect(result).toBeUndefined();
        });
    });

    describe('updateState', () => {
        it('should dispatch update action with data', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const updateData = { id: 'user-3', name: 'Test User 3' };

            service.updateState(updateData);

            expect(storeDispatchMock).toHaveBeenCalledWith({
                type: 'UPDATE_USER_DATA',
                payload: updateData,
            });
        });

        it('should call sliceIntf.update with the provided data', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const updateData = { roles: ['admin', 'user'] };

            service.updateState(updateData);

            expect(mockSliceIntf.update).toHaveBeenCalledWith(updateData);
        });

        it('should handle different data types in updateState', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            service.updateState(null);
            expect(mockSliceIntf.update).toHaveBeenCalledWith(null);

            service.updateState([1, 2, 3]);
            expect(mockSliceIntf.update).toHaveBeenCalledWith([1, 2, 3]);

            service.updateState({ nested: { object: { structure: true } } });
            expect(mockSliceIntf.update).toHaveBeenCalledWith({
                nested: { object: { structure: true } },
            });
        });
    });

    describe('broadcastInitEvent', () => {
        it('should publish init event with current state', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            service.broadcastInitEvent();

            expect(mockMessageService.publishInitEvent).toHaveBeenCalledWith({
                id: 'user-1',
                name: 'Test User',
            });
        });

        it('should call getState to retrieve current state', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const getStateSpy = jest.spyOn(service, 'getState');

            service.broadcastInitEvent();

            expect(getStateSpy).toHaveBeenCalled();
            getStateSpy.mockRestore();
        });

        it('should handle broadcastInitEvent with empty state', () => {
            storeGetStateMock.mockReturnValue({
                userData: {},
            });

            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);

            service.broadcastInitEvent();

            expect(mockMessageService.publishInitEvent).toHaveBeenCalledWith({});
        });
    });

    describe('integration scenarios', () => {
        it('should handle complete flow: init -> state change -> cleanup', () => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            const storeSubscriber = storSubscribeMock.mock.calls[0][0];

            // Broadcast initial state
            service.broadcastInitEvent();
            expect(mockMessageService.publishInitEvent).toHaveBeenCalled();

            // Simulate state change
            storeGetStateMock.mockReturnValue({
                userData: { id: 'updated-user', name: 'Updated' },
            });
            storeSubscriber();
            expect(mockMessageService.publishUpdateEvent).toHaveBeenCalled();

            // Cleanup
            service.cleanup();
            expect(unsubscribeMock).toHaveBeenCalled();
        });

        it('should prevent circular updates between message bus and Redux store', (done) => {
            const service = new DomainServiceBase(mockMessageService, mockSliceIntf);
            mockMessageService.publishUpdateEvent.mockClear();

            // Get the update handler
            const updateHandler = mockMessageService.subscribeToUpdateEvent.mock.calls[0][0];

            // Simulate receiving an event from message bus
            const eventDetail = { id: 'external-update' };
            const customEvent = new CustomEvent('update', { detail: eventDetail });

            updateHandler(customEvent);

            // This should have called updateState, which dispatches to store
            expect(storeDispatchMock).toHaveBeenCalled();

            // The publish flag should prevent re-publishing this update
            setTimeout(() => {
                expect((service as any).isPublishing).toBe(false);
                done();
            }, 10);
        });
    });
});
