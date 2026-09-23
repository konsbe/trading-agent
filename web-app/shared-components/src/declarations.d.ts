declare module '*.css' {
  const styles: { [className: string]: string };
  export default styles;
}

interface MfeMessageService {
  subscribe(componentName: string, handler: (event: Event) => void): () => void;
  publish(data: any): void;
}

declare module 'shellSpog/userDataMessageService' {
  export interface UserData {
    userName?: string;
    token?: string;
    userRoles?: string[];
    id?: string;
    email?: string;
    [key: string]: any;
  }
  export const mfeUserDataMessageService: MfeMessageService;
}

declare module 'shellSpog/filterDataMessageService' {
  export interface FilterData {
    [key: string]: any;
  }
  export const mfeFilterDataMessageService: MfeMessageService;
}
