/**
 * Interface for data domain services
 * Defines the contract for domain-specific data management and event handling
 */
export interface IDataDomainService {
  /**
   * Subscribes to data domain service events and updates
   */
  subscribe(): void;

  /**
   * Updates the state of the data domain service
   * @param data - The data to update the state with
   */
  updateState(data: unknown): void;

  /**
   * Retrieves the current state from the data domain service
   * @returns The current state
   */
  getState(): unknown;

  /**
   * Broadcasts an initialization event to notify listeners
   */
  broadcastInitEvent(): void;
}


export interface MfeOrchestrationService {
    init(): void;
    registerDataService(dataDomainService: IDataDomainService): void;
    registerMount(): void;
}
