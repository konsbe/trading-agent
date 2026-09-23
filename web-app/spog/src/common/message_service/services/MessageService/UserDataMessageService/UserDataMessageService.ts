import MessageServiceBase from "../MessageServiceBase";

const dataDomainId = 'user';

export class UserDataMessageService extends MessageServiceBase {
    private static instance: UserDataMessageService | null = null;

    private constructor() {
        super(dataDomainId);
    }

    public static getInstance(): UserDataMessageService {
        if (!UserDataMessageService.instance) {
            UserDataMessageService.instance = new UserDataMessageService();
        }
        return UserDataMessageService.instance;
    }
}