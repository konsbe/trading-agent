// Global type declarations for mfe-backtest-lab

declare module '*.module.css' {
  const classes: { [key: string]: string };
  export default classes;
}

declare module '*.css';

declare module '*.svg' {
  import React from 'react';
  const ReactComponent: React.FunctionComponent<React.SVGProps<SVGSVGElement>>;
  export { ReactComponent };
  const src: string;
  export default src;
}

declare module '*.png' { const src: string; export default src; }
declare module '*.jpg' { const src: string; export default src; }
declare module '*.jpeg' { const src: string; export default src; }
declare module '*.gif' { const src: string; export default src; }
declare module '*.webp' { const content: string; export default content; }

interface ShellSpogConfig {
  spogDomain?: string;
  momentumApiUrl?: string;
  [key: string]: unknown;
}

interface AppConfig {
  shell_spog?: {
    endpoint?: string;
    config?: ShellSpogConfig;
    [key: string]: unknown;
  };
  mfes?: Record<string, unknown>;
  [key: string]: unknown;
}

interface Window {
  __APP_CONFIG__?: AppConfig;
}
