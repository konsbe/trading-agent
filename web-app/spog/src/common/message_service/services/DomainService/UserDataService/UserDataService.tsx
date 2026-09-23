import { userDataSlice } from '../../../../state_management/slices/userData/userDataSlice';
import { UserDataMessageService } from '../../MessageService/UserDataMessageService/UserDataMessageService';
import DomainServiceBase from '../DomainServiceBase';
import mfeOrchestrationService from '../../MfeOrchestrationService';

export class UserDataService extends DomainServiceBase {
    private static instance: UserDataService | null = null;
    private isServiceInitialized: boolean = false;

    private constructor() {
        // Get the message service instance 
        const userDataMessageService = UserDataMessageService.getInstance();
        const sliceWithUpdate = {
            ...userDataSlice,
            updateTheme: userDataSlice.actions.updateUserDataTheme,
            update: userDataSlice.actions.updateUserDataStoreToken,
            selector: (state: any) => state.user
        };
        super(userDataMessageService, sliceWithUpdate);
        // UserDataService singleton instance created
    }

    public static getInstance(): UserDataService {
        if (!UserDataService.instance) {
            // Lazy instantiation triggered       
            UserDataService.instance = new UserDataService();
        }
        return UserDataService.instance;
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
}
