import mfeOrchestrationService from '../../MfeOrchestrationService';

class MessageServiceBase {
    // The timer used to temporarily activate the init event type. This should probably be in a global constants file
    removeInitEventListenerTimerInSeconds = 2;
    eventTypeApp = 'trading-agent';
    initAction = 'init';
    updateAction = 'update';
    initEventType: string;
    updateEventType: string;
    dataDomain: string;

    constructor(dataDomain: string) {
        // created domain for: "user"
        this.dataDomain = dataDomain;
        this.updateEventType = this.eventTypeApp + ':' + dataDomain + ':' + this.updateAction; // This is the event type used when domain data is updated
        this.initEventType = this.eventTypeApp + ':' + dataDomain + ':' + this.initAction; // This is the event type used to send the initial domain data to newly mounted MFEs
        return this;
    }

    public publishUpdateEvent(detail: any) {
        const event = new CustomEvent(this.updateEventType, {detail});

        setTimeout(() => { 
            window.dispatchEvent(event); 
        }, 0);  // this asynchronously emits the update event type
    }

    public publishInitEvent(detail: any) {
        const event = new CustomEvent(this.initEventType, {detail});
        
        setTimeout(() => { 
            window.dispatchEvent(event); 
        }, 0);    // this asynchronously emits the init event type
    }

    // Non-hook version for use outside React components (e.g., from remote MFEs)
    public subscribe = (componentName: string, subscribeCallback: any): (() => void) => {
        // Subscribe to both event types
        this.subscribeToUpdateEvent(subscribeCallback);
        this.subscribeToInitEvent(subscribeCallback);

        // Notify orchestration service
        mfeOrchestrationService.registerMount();

        // Return cleanup function
        return () => {
            this.removeUpdateEventListener(subscribeCallback);
            this.removeInitEventListener(subscribeCallback);
        };
    };

    // this is public so services can subscribe to update events
    public subscribeToUpdateEvent(callback: any) {
        // Subscribing to UPDATE events: "trading-agent:user:update"
        window.addEventListener(this.updateEventType, callback);
    }

    // this is public so services can unsubscribe to update events
    public removeUpdateEventListener(callback: any) {
        window.removeEventListener(this.updateEventType, callback);
    }

    private subscribeToInitEvent(callback: any) {
        window.addEventListener(this.initEventType, callback);
        setTimeout(() => { this.removeInitEventListener(callback); }, (this.removeInitEventListenerTimerInSeconds * 1000))
    }

    private removeInitEventListener(callback: any) {
        window.removeEventListener(this.initEventType, callback);
    }

}
export default MessageServiceBase;