
export type SseEntry = {
    source: EventSource;
    retries: number;
    retryTimer?: ReturnType<typeof globalThis.setTimeout>;
};


export type EventSources = Record<string, SseEntry>;