import { useEffect, useState } from 'react';
import { sseService } from '../services/SseService';
import { isMfeMetadataUpdateEvent, type MfeOperatorUiEvent } from '../types/mfeOperatorEvents';

const DEFAULT_SSE_PATH = '/mfe_operator_events';

const getSseEndpoint = (): string => {
  // return `http://${window.location.hostname}:9090${DEFAULT_SSE_PATH}`; // for local deployment (via port-forward of event notification server to 9090)
  return `${window.location.origin}${DEFAULT_SSE_PATH}`;
};

export const useMfeOperatorConfigSse = () => {
  const [event, setEvent] = useState<MfeOperatorUiEvent | null>(null);

  useEffect(() => {
    const eventName = 'shell_spog_mfe_operator_events';
    const sseEndpoint = getSseEndpoint();

    const handler = async (data: string) => {
      if (!data) return;
      let evt: unknown;
      try {
        evt = JSON.parse(data) as unknown;
      } catch {
        // Some servers may emit non-JSON payloads; ignore.
        return;
      }

      // Server may send an envelope: { event: { ... } }
      if (evt && typeof evt === 'object' && 'event' in (evt as any)) {
        evt = (evt as any).event as unknown;
      }

      if (!isMfeMetadataUpdateEvent(evt)) {
        return;
      }

      // Ensure every event creates a distinct object reference for React deps.
      setEvent({ ...evt });
    };

    // [SSE] Subscribing to mfe operator events: sseEndpoint
    sseService.subscribe(eventName, sseEndpoint, handler);

    return () => {
      sseService.unsubscribe(eventName);
    };
  }, []);
  return { event };
};
