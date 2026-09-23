import { IDataDomainService, MfeOrchestrationService } from '../types';

/**
 * Creates an MFE Orchestration Service instance
 * Manages registration of data domain services and broadcasts initialization events
 * with a configurable buffer timer to debounce multiple mount notifications
 */
export const createMfeOrchestrationService = (
    broadcastBufferTimerInMilliseconds: number = 500
): MfeOrchestrationService => {
    let broadcastBufferTimerActive = false;
    const dataDomainServices: IDataDomainService[] = [];
    let isInitialized = false;

    const init = (): void => {
        if (isInitialized) {
            return;
        }
        // Initializing orchestration service
        isInitialized = true;
    };

    /**
     * Broadcasts initialization event to all registered data domain services
     */
    const broadcastInitEvent = (): void => {
        dataDomainServices.forEach((service) => service.broadcastInitEvent());
        broadcastBufferTimerActive = false;
    };

    /**
     * Registers a data domain service
     * @param dataDomainService - The service to register
     */
    const registerDataService = (dataDomainService: IDataDomainService): void => {
        dataDomainServices.push(dataDomainService);
    };

    /**
     * Registers a mount notification
     * When a mount notification comes in and there is no active mount buffer timer, start one.
     * At the end of the timer, broadcast INIT event on each domainService.
     */
    const registerMount = (): void => {
        if (!broadcastBufferTimerActive) {
            broadcastBufferTimerActive = true;
            setTimeout(() => {
                broadcastInitEvent();
            }, broadcastBufferTimerInMilliseconds);
        }
    };

    return {
        init,
        registerDataService,
        registerMount,
    };
};

const mfeOrchestrationService = createMfeOrchestrationService();
export default mfeOrchestrationService;