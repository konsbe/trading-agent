export type SseEntry = {
  source: EventSource;
  retries: number;
  retryTimer?: number;
};

export type EventSources = Record<string, SseEntry>;
