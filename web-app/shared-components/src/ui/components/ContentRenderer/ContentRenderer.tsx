import React, { Suspense, useState, useEffect } from 'react';
import {
    type ContentRendererProps,
    type LocalComponentRendererProps,
    type RemoteMicrofrontendRendererProps,
    type IframeRendererProps
} from '../../types/content-wrapper';


// ============================================================================
// LOCAL COMPONENT RENDERER
// ============================================================================
const LocalComponentRenderer = ({
    component: Component,
    componentProps = {},
    onLoad,
    loadingComponent,
    errorComponent
}: LocalComponentRendererProps) => {
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);

    useEffect(() => {
        try {
            setIsLoading(true);
            setError(null);
            onLoad?.();
        } catch (err) {
            setError(err instanceof Error ? err : new Error('Unknown error'));
        } finally {
            setIsLoading(false);
        }
    }, [onLoad]);

    if (error) {
        const renderedError = typeof errorComponent === 'function'
            ? (errorComponent as (error: Error) => React.ReactNode)(error)
            : errorComponent;
        return <div>{renderedError}</div>;
    }

    if (isLoading) {
        return <div>{loadingComponent}</div>;
    }

    return <Component {...componentProps} />;
};

// ============================================================================
// REMOTE MICROFRONTEND RENDERER
// ============================================================================
const RemoteMicrofrontendRenderer = ({
    component: Component,
    componentProps = {},
    onLoad,
    onError,
    mfeKey,
    registerActiveMfe,
    unregisterActiveMfe
}: RemoteMicrofrontendRendererProps) => {
    const [error, setError] = useState<Error | null>(null);
    const [loading, setLoading] = useState<boolean>(false);

    useEffect(() => {
        if (!mfeKey || !registerActiveMfe || !unregisterActiveMfe) return;
        registerActiveMfe(mfeKey);
        return () => unregisterActiveMfe(mfeKey);
    }, [mfeKey, registerActiveMfe, unregisterActiveMfe]);

    useEffect(() => {
        onLoad?.();
    }, [onLoad]);

    if (error) {
        onError?.(error);
        return null;
    }

    if (loading) {
        return null;
    }

    return <Component {...componentProps} />;
};

// ============================================================================
// IFRAME RENDERER
// ============================================================================
const IframeRenderer = ({
    src,
    title = 'Content iframe',
    sandbox = 'allow-same-origin allow-scripts allow-popups allow-forms',
    allow = 'autoplay; clipboard-write; encrypted-media; picture-in-picture; web-share',
    referrerPolicy = 'strict-origin-when-cross-origin',
    allowFullScreen = true,
    loadingComponent,
    errorComponent,
    style = {},
    className = '',
    onLoad,
    onError,
    ...iframeProps
}: IframeRendererProps) => {
    /**
     * Track the current state of the iframe load process
     * 'loading' -> initial state while iframe is loading
     * 'loaded' -> iframe successfully loaded
     * 'error' -> iframe failed to load (onError fired)
     */
    const [loadState, setLoadState] = useState('loading');

    useEffect(() => {
        setLoadState('loading');
    }, [src]);


    const handleLoad = () => {
        setLoadState('loaded');
        onLoad?.();
    };

    /**
     * Handle iframe load error
     * Transitions state from 'loading' to 'error'
     * Calls the onError callback prop if provided
     * Note: CORS restrictions may prevent this from firing in some cases
     */
    const handleError = () => {
        setLoadState('error');
        onError?.();
    };

    return (
        <div className={`iframe-loader ${className}`} style={style}>
            {/* Show loading component while iframe is loading */}
            {loadState === 'loading' && (
                <div className="iframe-loader-loading" style={{ width: '100%', height: '100%' }}>
                    {loadingComponent}
                </div>
            )}

            {/* Show error component if iframe failed to load */}
            {loadState === 'error' && (
                <div className="iframe-loader-error" style={{ width: '100%', height: '100%' }}>
                    {errorComponent}
                </div>
            )}

            {/* Render iframe - hidden until onLoad fires for smooth transition */}
            <iframe
                src={src}
                title={title}
                sandbox={sandbox}
                allow={allow}
                referrerPolicy={referrerPolicy}
                allowFullScreen={allowFullScreen}
                onLoad={handleLoad}
                onError={handleError}
                style={{
                    border: 'none',
                    display: loadState === 'loaded' ? 'block' : 'none',
                    width: '100%',
                    height: '100%',
                }}
                {...iframeProps}
            />
        </div>
    );
};



export const ContentRenderer = ({
    props,
    loadingComponent,
}: ContentRendererProps) => {
    if (props.type === 'local') {
        return (
            <LocalComponentRenderer
                component={props.component}
                componentProps={props.componentProps}
                onLoad={props.onLoad}
                loadingComponent={props.loadingComponent}
                errorComponent={props.errorComponent}
            />
        );
    }

    if (props.type === 'remote') {
        return (
            <Suspense fallback={loadingComponent}>
                <RemoteMicrofrontendRenderer
                    component={props.component}
                    componentProps={props.componentProps}
                    onLoad={props.onLoad}
                    onError={props.onError}
                    mfeKey={props.mfeKey}
                />
            </Suspense>
        );
    }

    if (props.type === 'iframe') {
        return (
            <IframeRenderer
                src={props.src}
                title={props.title}
                sandbox={props.sandbox}
                allow={props.allow}
                referrerPolicy={props.referrerPolicy}
                style={props.style}
                onLoad={props.onLoad}
            />
        );
    }

    return null;
};
