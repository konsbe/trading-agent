import { filterDataSlice } from '../../../../state_management/slices/filterData/filterDataSlice';
import { FilterDataMessageService } from '../../MessageService/FilterDataMessageService/FilterDataMessageService';
import DomainServiceBase from '../DomainServiceBase';
import mfeOrchestrationService from '../../MfeOrchestrationService';
import store from '../../../../state_management/store/dataStore';

export class FilterDataService extends DomainServiceBase {
    private static instance: FilterDataService | null = null;
    private isServiceInitialized: boolean = false;

    private constructor() {
        // Get the message service instance 
        const filterDataMessageService = FilterDataMessageService.getInstance();
        const sliceWithUpdate = {
            ...filterDataSlice,
            update: filterDataSlice.actions.updateFilterData,
            selector: (state: any) => state.filterData
        };
        super(filterDataMessageService, sliceWithUpdate);
        // FilterDataService singleton instance created
    }

    public static getInstance(): FilterDataService {
        if (!FilterDataService.instance) {
            // Lazy instantiation triggered       
            FilterDataService.instance = new FilterDataService();
        }
        return FilterDataService.instance;
    }

    public init() {
        if (!this.isServiceInitialized) {
            // Initializing User Data Service
            // Register with orchestration service
            mfeOrchestrationService.registerDataService(this);
            this.isServiceInitialized = true;
        } else {
            // Already initialized, skipping
        }
    }

    public updateState(data: any) {        
        store.dispatch(filterDataSlice.actions.updateFilterData( data ));   
    }
}

