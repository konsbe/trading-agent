import { ReactNode, ComponentType } from 'react';

export interface BaseContentProps {
    id: string;
    className?: string;
    style?: React.CSSProperties;
    loadingComponent?: ReactNode;
    errorComponent?: (error: Error) => ReactNode;
    onLoad?: () => void;
    onError?: (error: Error) => void;
    roles?: string[];
}

export interface LocalComponentProps extends BaseContentProps {
  type: 'local';
  component: ComponentType<any>;
  componentProps?: Record<string, any>;
}

export interface RemoteMicrofrontendProps extends BaseContentProps {
  type: 'remote';
  component: ComponentType<any>;
  componentProps?: Record<string, any>;
  mfeKey?: string;
}

export interface IframeProps extends BaseContentProps {
  type: 'iframe';
  src: string;
  title?: string;
  sandbox?: string;
  allow?: string;
  referrerPolicy?: 'no-referrer' | 'no-referrer-when-downgrade' | 'same-origin' | 'origin' | 'strict-origin' | 'origin-when-cross-origin' | 'strict-origin-when-cross-origin' | 'unsafe-url';
}

export type ContentWrapperProps = LocalComponentProps | RemoteMicrofrontendProps | IframeProps;

export interface ContentRendererProps {
  props: ContentWrapperProps;
  loadingComponent: ReactNode;
}

export interface LocalComponentRendererProps {
    component: ComponentType<any>;
    componentProps?: Record<string, any>;
    onLoad?: () => void;
    loadingComponent?: React.ReactNode;
    errorComponent?: React.ReactNode | ((error: Error) => React.ReactNode);
}

export interface RemoteMicrofrontendRendererProps {
    component: ComponentType<any>;
    componentProps?: Record<string, any>;
    onLoad?: () => void;
    onError?: (error: Error) => void;
  mfeKey?: string;
  registerActiveMfe?: (mfeKey: string) => void;
  unregisterActiveMfe?: (mfeKey: string) => void;
}

export interface IframeRendererProps {
    src: string;
    title?: string;
    sandbox?: string;
    allow?: string;
    referrerPolicy?: 'no-referrer' | 'no-referrer-when-downgrade' | 'same-origin' | 'origin' | 'strict-origin' | 'origin-when-cross-origin' | 'strict-origin-when-cross-origin' | 'unsafe-url';
    allowFullScreen?: boolean;
    loadingComponent?: React.ReactNode;
    errorComponent?: React.ReactNode;
    style?: React.CSSProperties;
    className?: string;
    onLoad?: () => void;
    onError?: () => void;
    [key: string]: any;
}
