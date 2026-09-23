import { UserDataMessageService } from "./UserDataMessageService";

// For backward compatibility - lazy singleton
export const userDataMessageService = UserDataMessageService.getInstance(); 
