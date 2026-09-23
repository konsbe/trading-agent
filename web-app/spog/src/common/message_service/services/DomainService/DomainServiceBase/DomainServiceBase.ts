import store from '../../../../state_management/store/dataStore';
import MessageServiceBase from '../../MessageService/MessageServiceBase';
import { IDataDomainService } from '../../../types';

/**
 * SliceIntf represents a Redux slice with update capabilities
 */
interface SliceIntf {
    name: string;
    update: (data: any) => any;
    selector: (state: any) => any;
}

class DomainServiceBase implements IDataDomainService {
    messageService: MessageServiceBase;
    sliceIntf: SliceIntf;
    slice: string;
    private previousState: any = null;
    private unsubscribeFromStore: (() => void) | null = null;
    private isPublishing: boolean = false;

    constructor(messageService: MessageServiceBase, sliceIntf: SliceIntf) {
        this.messageService = messageService;
        this.sliceIntf = sliceIntf;
        this.slice = sliceIntf.name;

        // Listen to any updates on the message bus and update the state store
        this.messageService.subscribeToUpdateEvent(this.updateHandler);
        // Subscribe to Redux store changes to publish to message bus
        this.subscribe();
    }

    /**
     * Handles update events from the message bus
     * Prevents infinite loops by ignoring events published by this instance
     */
    private updateHandler = (event: Event) => {
        const context = (event as CustomEvent).detail;

        // Ignore events that we just published ourselves
        if (this.isPublishing) {
            return;
        }

        // Update state when message bus UPDATE is received
        this.updateState(context);
    };

    /**
     * Subscribes to Redux store changes and publishes updates to the message bus
     * Uses debouncing to prevent excessive event publishing
     */
    public subscribe() {
        // Subscribe to Redux store to detect state changes
        this.unsubscribeFromStore = store.subscribe(() => {
            const currentState = this.getState();

            // Only publish if state actually changed
            if (JSON.stringify(currentState) !== JSON.stringify(this.previousState)) {
                // Set flag before publishing to ignore our own event
                this.isPublishing = true;
                this.messageService.publishUpdateEvent(currentState);

                // Reset flag after event is dispatched (async to allow event to propagate)
                setTimeout(() => {
                    this.isPublishing = false;
                }, 0);

                this.previousState = currentState;
            }
        });

        // Store initial state
        this.previousState = this.getState();
    }

    /**
     * Cleanup method to unsubscribe from store changes
     */
    public cleanup() {
        if (this.unsubscribeFromStore) {
            this.unsubscribeFromStore();
        }
    }

    /**
     * Retrieves the current state for this slice from the Redux store
     * @returns The current slice state
     */
    public getState() {
        const state = store.getState();
        return state[this.slice];
    }

    /**
     * Updates the state by dispatching an action to Redux
     * @param data The data to update the state with
     */
    public updateState(data: any) {
        // Dispatch actions to modify the store
        store.dispatch(this.sliceIntf.update(data));
    }

    /**
     * Emits an initialization event to notify listeners of the current state
     */
    public broadcastInitEvent() {
        const currentState = this.getState();
        // Emit INIT event with current state
        this.messageService.publishInitEvent(currentState);
    }
}

export default DomainServiceBase;