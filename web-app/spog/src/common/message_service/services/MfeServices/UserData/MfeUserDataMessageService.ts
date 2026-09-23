import { UserDataMessageService } from '../../MessageService/UserDataMessageService/UserDataMessageService';
import { getUserDataService } from '../../DomainService/UserDataService';

export class MfeUserDataMessageService {
    private static instance: MfeUserDataMessageService | null = null;
    private readonly updateEventType: string;
    private readonly initEventType: string;
    private readonly userDataMessageService: UserDataMessageService;

    private constructor() {
        this.updateEventType = 'trading-agent:user:update';
        this.initEventType = 'trading-agent:user:init';
        this.userDataMessageService = UserDataMessageService.getInstance();
    }

    public static getInstance(): MfeUserDataMessageService {
        if (!MfeUserDataMessageService.instance) {
            MfeUserDataMessageService.instance = new MfeUserDataMessageService();
        }
        return MfeUserDataMessageService.instance;
    }

    public subscribe(componentName: string, handler: (event: Event) => void): () => void {
        window.addEventListener(this.updateEventType, handler);
        window.addEventListener(this.initEventType, handler);

        const userDataService = getUserDataService();
        const currentState = userDataService.getState();

        if (currentState) {
            this.userDataMessageService.publishInitEvent(currentState);
        }

        return () => {
            window.removeEventListener(this.updateEventType, handler);
            window.removeEventListener(this.initEventType, handler);
        };
    }

    public publish(data: any): void {
        this.userDataMessageService.publishUpdateEvent(data);
    }
}

