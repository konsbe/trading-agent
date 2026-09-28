// Remote modules exposed by the spog shell (MF container `shell_spog`, remote alias `shellSpog`).

interface MfeMessageService {
  /** Subscribe to shell events; returns the unsubscribe function. */
  subscribe(componentName: string, handler: (event: Event) => void): () => void;
  publish(data: unknown): void;
}

declare module 'shellSpog/userDataMessageService' {
  export interface UserData {
    userName?: string;
    token?: string;
    userRoles?: string[];
    authenticated?: boolean;
    theme?: 'light' | 'dark';
    [key: string]: unknown;
  }

  export const mfeUserDataMessageService: MfeMessageService;
}

// Not used by this MFE, but referenced by the shared-components barrel.
declare module 'shellSpog/filterDataMessageService' {
  export interface FilterData {
    [key: string]: unknown;
  }

  export const mfeFilterDataMessageService: MfeMessageService;
}
