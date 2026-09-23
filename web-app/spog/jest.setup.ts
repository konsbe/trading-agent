import '@testing-library/jest-dom';
// --- Add jest-fetch-mock setup ---
import { TextEncoder, TextDecoder } from 'util';

process.env.IS_JEST_ENV = 'true';

// jsdom does not implement window.matchMedia; provide a minimal stub so that
// modules evaluated at import time (e.g. userDataSlice) don't throw.
Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        dispatchEvent: jest.fn(),
    }),
});

// Declare global if needed
declare global {
    // Only declare if not already present (Node 18+ includes them by default)
    var TextEncoder: typeof TextEncoder;
    var TextDecoder: typeof TextDecoder;
}

(global as any).TextEncoder = TextEncoder;
(global as any).TextDecoder = TextDecoder;

// jsdom does not provide EventSource; mock it so SSE-related hooks/providers can mount.
class MockEventSource {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSED = 2;

  url: string;
  readyState: number = MockEventSource.OPEN;
  withCredentials: boolean = false;
  onopen: ((ev: Event) => any) | null = null;
  onmessage: ((ev: MessageEvent) => any) | null = null;
  onerror: ((ev: Event) => any) | null = null;

  constructor(url: string) {
    this.url = String(url);
    // Simulate async connection establishment.
    setTimeout(() => {
      this.onopen?.(new Event('open'));
    }, 0);
  }

  close() {
    this.readyState = MockEventSource.CLOSED;
  }

  // Utility for tests if they want to push messages.
  emitMessage(data: string) {
    this.onmessage?.(new MessageEvent('message', { data }));
  }

  emitError() {
    this.onerror?.(new Event('error'));
  }
}

(global as any).EventSource = MockEventSource as any;

const RealXMLHttpRequest = window.XMLHttpRequest;

(window as any).XMLHttpRequest = class MockXMLHttpRequest {
  _method: string | undefined;
  _url: string | undefined;
  _requestHeaders: Record<string, string> = {};
  _listeners: Record<string, Function[]> = {};
  readyState = 0; // Start as UNSENT
  status = 0;
  responseText = '';
  onreadystatechange: (() => any) | null = null;
  onerror: (() => any) | null = null; // Add onerror handler

  constructor() {
    // Log creation immediately
    console.info('[XHR DEBUG - CONSTRUCTOR] New XMLHttpRequest instance created');
    // It's hard to get the call stack or origin here easily
  }

  open = jest.fn((method: string, url: string, async: boolean = true, user?: string, password?: string) => {
    // Log open immediately
    console.info(`[XHR DEBUG - OPEN] Method: ${method}, URL: ${url}, Async: ${async}`);
    this._method = method;
    this._url = url;
    this.readyState = 1; // OPENED
    this.status = 0; // Status is 0 before send
    this.responseText = '';
    this.onreadystatechange?.(); // Trigger state change

    // *Crucially*: Check if this is the problematic URL immediately
    if (this._url?.startsWith('http://127.0.0.1:80')) {
      console.info(`[XHR DEBUG - OPEN] >>> Detected problematic URL: ${this._url}`);
      // Optionally, trigger an error here to prevent the underlying connection attempt
      // and see if it stops the ECONNREFUSED.
      // This requires simulating the error lifecycle.
      // Let's just log for now to see if this line is hit for 127.0.0.1:80.
    }
  });

  send = jest.fn((body?: Document | XMLHttpRequestBodyInit | null) => {
    console.info(`[XHR DEBUG - SEND] Sending request to ${this._url}`, body);

    // Simulate a response (or error) here based on the URL
    // If you uncomment the error simulation in open, you might not reach here for 127.0.0.1:80

    if (this._url?.startsWith('http://127.0.0.1:80')) {
      console.info('[XHR DEBUG - SEND] Simulating error response for 127.0.0.1:80');
      // Simulate a network error
      this.readyState = 4; // DONE
      this.status = 0; // Network error typically has status 0 or is handled before status is set
      this.onreadystatechange?.();
      const errorEvent = new Event('error');
      // Manually trigger the error handler
      this.onerror?.();
      this._listeners['error']?.forEach(listener => (listener as any)(errorEvent));

    } else if (this._url?.includes('test-file-stub')) {
      console.info('[XHR DEBUG - SEND] Simulating success for test-file-stub');
      // Simulate success for the file stub
      this.readyState = 4; // DONE
      this.status = 200;
      this.responseText = 'mock-file-content'; // Provide some mock content

      this.onreadystatechange?.();
      const loadEvent = new Event('load');
      this._listeners['load']?.forEach(listener => (listener as any)(loadEvent));

    } else {
      console.info(`[XHR DEBUG - SEND] Simulating success for unhandled URL: ${this._url}`);
      // Default success for others
      this.readyState = 4; // DONE
      this.status = 200;
      this.responseText = '{}';

      this.onreadystatechange?.();
      const loadEvent = new Event('load');
      this._listeners['load']?.forEach(listener => (listener as any)(loadEvent));
    }
  })

  // ... (setRequestHeader, addEventListener, removeEventListener, abort, getResponseHeader, etc. - keep the ones from the previous mock) ...

  // Add onerror property
};