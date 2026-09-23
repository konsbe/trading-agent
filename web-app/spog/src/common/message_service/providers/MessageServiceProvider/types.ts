import { getFilterDataService } from "../../services/DomainService/FilterDataService";
import { getUserDataService } from "../../services/DomainService/UserDataService";
import mfeOrchestrationService from "../../services/MfeOrchestrationService";

export type MessageServiceProviderProps = {
    mfeOrchestrationService: typeof mfeOrchestrationService;
    userDataService: ReturnType<typeof getUserDataService>;
    filterDataService: ReturnType<typeof getFilterDataService>;
}
