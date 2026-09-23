import { SseService } from './sse';

const sseService = SseService.getInstance();

export { sseService };
export type { EventSources, SseEntry } from './types';
