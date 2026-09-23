import React, { useEffect, createContext, useContext, ReactNode, useRef } from "react";
import { type MessageServiceProviderProps } from "./types";
import mfeOrchestrationService from '../../services/MfeOrchestrationService';
import { getUserDataService } from "../../services/DomainService/UserDataService";
import { getFilterDataService } from "../../services/DomainService/FilterDataService";


export const MessageServiceContext = createContext<MessageServiceProviderProps | undefined>(undefined);

// Global flag to ensure services are only initialized once across all instances
let servicesInitialized = false;

/**
 * MessageServiceProvider - Initializes the MFE communication infrastructure
 * 
 * This provider ensures that:
 * 1. The MFE Orchestration Service is initialized
 * 2. All domain data services are initialized and registered
 * 3. Services are only initialized once during the singleton pattern
 * 4. Services are initialized AFTER Redux store is available
 * 
 * Bootstrap MessageServiceProvider
 *  [MessageServiceBase] constructor Created domain for: "user"
 *  [UserDataService] getInstance Lazy instantiation triggered
 *  [MessageServiceBase] subscribeToUpdateEvent Subscribing to UPDATE events: "trading-agent:user:update"
 *  [DataServiceBase] constructor Subscribed to UPDATE events for slice: "user"
 *  [UserDataService] constructor UserDataService singleton instance created
 *  [MessageServiceProvider] mfeOrchestrationService.init(); Initializing MFE Orchestration Service
 *  [MfeOrchestrationService] init Initializing orchestration service
 *  [MessageServiceProvider] mfeOrchestrationService.init();  MFE Orchestration Service initialized
 *  [UserDataService] init Initializing User Data Service
 *  [MfeOrchestrationService] registerDataService Data service registered.Total services: 1
 *  [UserDataService] mfeOrchestrationService.registerDataService(this); Registered with orchestration service
 *  [MessageServiceProvider] userDataService.init();  User Data Service initialized
 *
 * Subscribe to events from spog to singleton
 *  [DataServiceBase] constructor this.subscribeToStoreChanges(); Subscribed to message bus UPDATE events for slice: "user"
 *  [DataServiceBase] subscribeToStoreChanges this.previousState = this.getState();  Subscribed to Redux store changes for slice: "user"
 *  [DataServiceBase] subscribeToStoreChanges const currentState = this.getState();  Redux state changed for "user", publishing UPDATE event: {currentUser: {…}, isAuthenticated: true}
 *  [DataServiceBase] updateHandler this.isPublishing Ignoring own published event for "user"
 */
export const MessageServiceProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const isInitialized = useRef(false);

    useEffect(() => {
        // Double-check: both local ref and global flag must be false
        if (!isInitialized.current && !servicesInitialized) {
            // Initializing MFE communication services...');

            try {
                // Initialize MFE Orchestration Service
                mfeOrchestrationService.init();

                // Get User Data Service instance (lazy singleton)
                const userDataService = getUserDataService();
                const filterDataService = getFilterDataService();

                // Initialize Service
                userDataService.init();
                filterDataService.init();
                //registered services

                // Mark as initialized
                isInitialized.current = true;
                servicesInitialized = true;
            } catch (error) {
                // Error initializing services
            }
        } else if (servicesInitialized) {
            // Services already initialized globally, skipping
        }

        return () => {
            //  Provider unmounting (services remain active)
        };
    }, []);

    const contextValue = {
        mfeOrchestrationService,
        userDataService: getUserDataService(),
        filterDataService: getFilterDataService(),
    };

    return (
        <MessageServiceContext.Provider value={contextValue}>
            {children}
        </MessageServiceContext.Provider>
    );
};

export const useMessageService = (): MessageServiceProviderProps => {
    const context = useContext(MessageServiceContext);
    if (!context) {
        throw new Error("useMessageService must be used within a MessageServiceProvider");
    }
    return context;
};

export default MessageServiceProvider;