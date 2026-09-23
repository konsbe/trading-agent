import { createMfeOrchestrationService } from './MfeOrchestrationService';
import { IDataDomainService } from '../types';

describe('MfeOrchestrationService', () => {
  let mfeOrchestrationService: ReturnType<typeof createMfeOrchestrationService>;

  beforeEach(() => {
    mfeOrchestrationService = createMfeOrchestrationService();
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.runOnlyPendingTimers();
    jest.useRealTimers();
  });

  describe('broadcastInitEvent', () => {
    it('should call broadcastInitEvent on all registered data domain services', () => {
      // Arrange
      const mockDataDomainService1: IDataDomainService = {
        subscribe: jest.fn(),
        updateState: jest.fn(),
        getState: jest.fn(),
        broadcastInitEvent: jest.fn(),
      };

      const mockDataDomainService2: IDataDomainService = {
        subscribe: jest.fn(),
        updateState: jest.fn(),
        getState: jest.fn(),
        broadcastInitEvent: jest.fn(),
      };

      // Register the mock services
      mfeOrchestrationService.registerDataService(mockDataDomainService1);
      mfeOrchestrationService.registerDataService(mockDataDomainService2);

      // Act
      mfeOrchestrationService.registerMount();
      jest.advanceTimersByTime(500);

      // Assert
      expect(mockDataDomainService1.broadcastInitEvent).toHaveBeenCalled();
      expect(mockDataDomainService2.broadcastInitEvent).toHaveBeenCalled();
    });

    it('should call broadcastInitEvent on each service exactly once per registerMount call', () => {
      // Arrange
      const mockDataDomainService: IDataDomainService = {
        subscribe: jest.fn(),
        updateState: jest.fn(),
        getState: jest.fn(),
        broadcastInitEvent: jest.fn(),
      };

      mfeOrchestrationService.registerDataService(mockDataDomainService);

      // Act
      mfeOrchestrationService.registerMount();
      jest.advanceTimersByTime(500);

      // Assert
      expect(mockDataDomainService.broadcastInitEvent).toHaveBeenCalledTimes(1);
    });

    it('should buffer multiple registerMount calls and broadcast only once', () => {
      // Arrange
      const mockDataDomainService: IDataDomainService = {
        subscribe: jest.fn(),
        updateState: jest.fn(),
        getState: jest.fn(),
        broadcastInitEvent: jest.fn(),
      };

      mfeOrchestrationService.registerDataService(mockDataDomainService);

      // Act
      mfeOrchestrationService.registerMount();
      mfeOrchestrationService.registerMount();
      mfeOrchestrationService.registerMount();
      jest.advanceTimersByTime(500);

      // Assert - should only be called once despite multiple registerMount calls
      expect(mockDataDomainService.broadcastInitEvent).toHaveBeenCalledTimes(1);
    });

    it('should not call broadcastInitEvent before timer completes', () => {
      // Arrange
      const mockDataDomainService: IDataDomainService = {
        subscribe: jest.fn(),
        updateState: jest.fn(),
        getState: jest.fn(),
        broadcastInitEvent: jest.fn(),
      };

      mfeOrchestrationService.registerDataService(mockDataDomainService);

      // Act
      mfeOrchestrationService.registerMount();
      jest.advanceTimersByTime(100); // Less than 500ms

      // Assert
      expect(mockDataDomainService.broadcastInitEvent).not.toHaveBeenCalled();

      // Act - advance remaining time
      jest.advanceTimersByTime(400);

      // Assert
      expect(mockDataDomainService.broadcastInitEvent).toHaveBeenCalled();
    });
  });

  describe('registerDataService', () => {
    it('should register a data domain service', () => {
      // Arrange
      const mockDataDomainService: IDataDomainService = {
        subscribe: jest.fn(),
        updateState: jest.fn(),
        getState: jest.fn(),
        broadcastInitEvent: jest.fn(),
      };

      // Act
      mfeOrchestrationService.registerDataService(mockDataDomainService);
      mfeOrchestrationService.registerMount();
      jest.advanceTimersByTime(500);

      // Assert
      expect(mockDataDomainService.broadcastInitEvent).toHaveBeenCalled();
    });
  });
});
