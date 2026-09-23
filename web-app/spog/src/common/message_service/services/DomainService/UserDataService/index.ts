import { UserDataService } from "./UserDataService";

// Export getter function
export const getUserDataService = (): UserDataService => UserDataService.getInstance();