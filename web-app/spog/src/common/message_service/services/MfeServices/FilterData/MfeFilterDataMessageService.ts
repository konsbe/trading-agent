import { FilterDataMessageService } from '../../MessageService/FilterDataMessageService/FilterDataMessageService';
import { getFilterDataService } from '../../DomainService/FilterDataService';

export class MfeFilterDataMessageService {
    private static instance: MfeFilterDataMessageService | null = null;
    private readonly updateEventType: string;
    private readonly initEventType: string;
    private readonly filterDataMessageService: FilterDataMessageService;

    private constructor() {
        this.updateEventType = 'trading-agent:filter:update';
        this.initEventType = 'trading-agent:filter:init';
        this.filterDataMessageService = FilterDataMessageService.getInstance();
    }

    public static getInstance(): MfeFilterDataMessageService {
        if (!MfeFilterDataMessageService.instance) {
            MfeFilterDataMessageService.instance = new MfeFilterDataMessageService();
        }
        return MfeFilterDataMessageService.instance;
    }

    public subscribe(componentName: string, handler: (event: Event) => void): () => void {
        window.addEventListener(this.updateEventType, handler);
        window.addEventListener(this.initEventType, handler);

        const filterDataService = getFilterDataService();
        const currentState = filterDataService.getState();

        if (currentState) {
            this.filterDataMessageService.publishInitEvent(currentState);
        }

        return () => {
            window.removeEventListener(this.updateEventType, handler);
            window.removeEventListener(this.initEventType, handler);
        };
    }
    
    public publish(data: any): void {
        this.filterDataMessageService.publishUpdateEvent(data);
    }

}
