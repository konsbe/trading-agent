/***************************************************
 * SSE Service with Controlled Retries
 * - Manages EventSource instances per client key
 * - Provides controlled retry/backoff (bypasses native EventSource retry)
 ***************************************************/

import { EventSources } from './types';

const eventSources: EventSources = {};

const MAX_RETRIES = 5;
const BASE_RETRY_DELAY_MS = 3000;
const MAX_RETRY_DELAY_MS = 30000;

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max);

export class SseService {
  private static instance: SseService | null = null;

  public static getInstance(): SseService {
    if (!SseService.instance) {
      SseService.instance = new SseService();
    }
    return SseService.instance;
  }

  public subscribe(eventName: string, sseEndpoint: string, onRcvHandler: (data: string) => void) {
    if (!eventName || !sseEndpoint) {
      console.warn('[SSE] subscribe called with empty args', { eventName, sseEndpoint });
      return;
    }

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
      window.clearTimeout(entry.retryTimer);
      entry.retryTimer = undefined;
    }

    entry.source.close();
    delete eventSources[eventName];
  }

  private connect(eventName: string, sseEndpoint: string, onRcvHandler: (data: string) => void) {
    const sseClient = new EventSource(sseEndpoint);

    if (!eventSources[eventName]) {
      eventSources[eventName] = {
        source: sseClient,
        retries: 0,
      };
    } else {
      eventSources[eventName].source = sseClient;
    }

    sseClient.onopen = () => {
      console.info(`[SSE] Connected: ${eventName}`);
      eventSources[eventName].retries = 0;
    };

    sseClient.onmessage = (sseEvent) => {
      try {
        onRcvHandler(String(sseEvent.data));
      } catch (e) {
        console.error('[SSE] onRcvHandler failed', e);
      }
    };

    sseClient.onerror = () => {
      console.warn(`[SSE] Error: ${eventName}`);
      this.handleRetry(eventName, sseEndpoint, onRcvHandler);
    };
  }

  private handleRetry(eventName: string, sseEndpoint: string, onRcvHandler: (data: string) => void) {
    const entry = eventSources[eventName];
    if (!entry) return;

    // Bypass browser default retry mechanism for more control.
    entry.source.close();

    if (entry.retries >= MAX_RETRIES) {
      console.warn(`[SSE] Max retries reached: ${eventName}`);
      delete eventSources[eventName];
      return;
    }

    if (entry.retryTimer) {
      window.clearTimeout(entry.retryTimer);
      entry.retryTimer = undefined;
    }

    const delay = clamp(BASE_RETRY_DELAY_MS * Math.pow(2, entry.retries), BASE_RETRY_DELAY_MS, MAX_RETRY_DELAY_MS);
    entry.retries++;

    console.info(`[SSE] Retry ${entry.retries}/${MAX_RETRIES} in ${delay}ms: ${eventName}`);

    entry.retryTimer = window.setTimeout(() => {
      this.connect(eventName, sseEndpoint, onRcvHandler);
    }, delay);
  }
}
