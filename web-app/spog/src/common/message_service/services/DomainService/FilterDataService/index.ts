import { FilterDataService } from "./FilterDataService";

// Export getter function
export const getFilterDataService = (): FilterDataService => FilterDataService.getInstance();