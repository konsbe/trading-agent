import { SseService } from './sse';

export const sseService = SseService.getInstance();
export { SseService };
export type { EventSources, SseEntry } from './types';
