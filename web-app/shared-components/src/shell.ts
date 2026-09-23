/**
 * SPOG shell entry — ContentWrapper, primitives and related UI only.
 * Do not export MFE hooks (useAuthMFE / useFilterData); they import shellSpog remotes
 * and cannot resolve inside the shell bundle.
 */
export { ContentWrapper } from './ui/components/ContentWrapper/ContentWrapper';
export { ContentRenderer } from './ui/components/ContentRenderer/ContentRenderer';
export { default as ErrorBoundary } from './ui/components/ErrorBoundary/ErrorBoundary';
export { default as FullSizeSkeleton } from './ui/components/Skeleton/FullSizeSkeleton';
export { default as ThemeProvider, getSystemTheme } from './ui/providers/ThemeProvider/ThemeProvider';
export type { ThemeMode } from './ui/providers/ThemeProvider/types';
export { getThemeVariables, applyTheme, THEME_MODES } from './theme/tokens';
export type { StitchColorToken } from './theme/tokens';
export * from './ui/primitives';
export type {
    ContentWrapperProps,
    ContentRendererProps,
    LocalComponentProps,
    RemoteMicrofrontendProps,
    IframeProps,
    BaseContentProps,
} from './ui/types/content-wrapper';
