/***************************************************
 * SSE Service with Controlled Retries
 ***************************************************/

import { EventSources } from "./types";

const eventSources: EventSources = {};

const MAX_RETRIES = 5;
const RETRY_DELAY_MS = 3000;

export class SseService {

    private static instance: SseService | null = null;

    public static getInstance(): SseService {
        SseService.instance ??= new SseService();
        return SseService.instance;
    }

    public subscribe(eventName: string, sseEndpoint: string, onRcvHandler: any){ // eventName = clientID (e.g. topology_mfe, topology_groups_mfe)

        if (eventSources[eventName]) {
            return;
        }

        this.connect(eventName, sseEndpoint, onRcvHandler);
    }

    public unsubscribe(eventName: string): void {
        const entry = eventSources[eventName];
        if (!entry) {
            return;
        }

        if (entry.retryTimer) {
            globalThis.clearTimeout(entry.retryTimer);
            entry.retryTimer = undefined;
        }

        entry.source.close();
        delete eventSources[eventName];
    }

    private connect(eventName: string, sseEndpoint: string, onRcvHandler: any) {
        const sseClient = new EventSource(sseEndpoint);

        if (eventSources[eventName]) {
            eventSources[eventName].source = sseClient;
        } else {
            eventSources[eventName] = {
                source: sseClient,
                retries: 0
            };
        }

        sseClient.onopen = () => {
            eventSources[eventName].retries = 0;
        };

        sseClient.onmessage = (sseEvent) => {
            onRcvHandler(sseEvent.data);
        };

        sseClient.onerror = () => {
            this.handleRetry(eventName, sseEndpoint, onRcvHandler);
        };
    }

    private handleRetry(eventName: string, sseEndpoint: string, onRcvHandler: any) {

        const entry = eventSources[eventName];
        if (!entry) return;

        entry.source.close();

        if (entry.retries >= MAX_RETRIES) {
            delete eventSources[eventName];
            return;
        }

        if (entry.retryTimer) {
            globalThis.clearTimeout(entry.retryTimer);
            entry.retryTimer = undefined;
        }

        const delay = RETRY_DELAY_MS;
        entry.retries++;

        entry.retryTimer = globalThis.setTimeout(() => {
            this.connect(eventName, sseEndpoint, onRcvHandler);
        }, delay);
    }
}
