import MessageServiceBase from "../MessageServiceBase";

const dataDomainId = 'filter';

export class FilterDataMessageService extends MessageServiceBase {
    private static instance: FilterDataMessageService | null = null;

    private constructor() {
        super(dataDomainId);
    }

    public static getInstance(): FilterDataMessageService {
        if (!FilterDataMessageService.instance) {
            FilterDataMessageService.instance = new FilterDataMessageService();
        }
        return FilterDataMessageService.instance;
    }
}