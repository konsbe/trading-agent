import React from 'react';
import addOnComponent from './assets/add-on-empty-component.svg';

export interface MFEDataWrapperProps {
  children: React.ReactNode;
  dataLoading?: boolean;
  dataError?: Error | null;
  /** Used for empty-state detection when `noDataDeactivated` is false (aligned with mfe-topology-groups). */
  data?: unknown[] | boolean | null;
  noDataMessage?: string;
  /** Typo alias from mfe-topology-groups — prefer `noDataMessage`. */
  nodDataMessage?: string;
  /** When false, empty state shows only `noDataMessage` (no illustration). */
  showEmptyIllustration?: boolean;
  /** When true, skip the empty-data placeholder and always render `children` after load. */
  noDataDeactivated?: boolean;
  /** Merged into the empty-state illustration + message stack. */
  emptyStateStackStyle?: React.CSSProperties;
  loadingMessage?: string;
  /** Legacy alias — prefer `dataLoading`. */
  isLoading?: boolean;
  /** Legacy alias — prefer `dataError`. */
  isError?: boolean;
  /** Legacy alias — prefer `dataError`. */
  errorMessage?: string | Error | null;
  /** Legacy alias — prefer `noDataDeactivated` or pass non-empty `data`. */
  dataExist?: boolean;
}

function isNoData(data: unknown): boolean {
  if (data == null) return true;
  if (typeof data === 'boolean') return !data;
  if (Array.isArray(data)) {
    if (data.length === 0) return true;
    const first = data[0] as Record<string, unknown> | undefined;
    if (first && typeof first === 'object' && 'name' in first) {
      return !first.name;
    }
    return false;
  }
  return false;
}

function toDataError(
  dataError: Error | null | undefined,
  isError: boolean | undefined,
  errorMessage: string | Error | null | undefined
): Error | null {
  if (dataError) return dataError;
  if (!isError) return null;
  if (errorMessage instanceof Error) return errorMessage;
  if (typeof errorMessage === 'string' && errorMessage.length > 0) {
    return new Error(errorMessage);
  }
  return new Error('Failed to fetch data');
}

/**
 * Loading / error / empty states — aligned with mfe-trace-configurator and mfe-topology-groups.
 */
const MFEDataWrapper: React.FC<MFEDataWrapperProps> = ({
  children,
  dataLoading,
  dataError,
  data,
  noDataMessage,
  nodDataMessage,
  showEmptyIllustration = true,
  noDataDeactivated,
  emptyStateStackStyle,
  loadingMessage = 'Loading data...',
  isLoading,
  isError,
  errorMessage,
  dataExist,
}) => {
  const resolvedLoading = dataLoading ?? isLoading ?? false;
  const resolvedError = toDataError(dataError, isError, errorMessage ?? null);
  const resolvedNoDataMessage = noDataMessage ?? nodDataMessage;
  const resolvedNoDataDeactivated = noDataDeactivated === true || dataExist === true;

  if (resolvedLoading) {
    return (
      <div
        style={{
          padding: '20px',
          textAlign: 'center',
          color: 'var(--color-text-secondary)',
        }}
      >
        {loadingMessage}
      </div>
    );
  }

  if (resolvedError) {
    return (
      <div
        style={{
          padding: '20px',
          textAlign: 'center',
          color: 'var(--color-error)',
        }}
      >
        There was a problem trying to fetch your data.
      </div>
    );
  }

  const showNoData = !resolvedNoDataDeactivated && isNoData(data);
  if (showNoData) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
        }}
      >
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            alignItems: 'center',
            ...emptyStateStackStyle,
          }}
        >
          {showEmptyIllustration ? <img src={addOnComponent} alt="" /> : null}
          <span style={{ color: 'var(--color-text-secondary)' }}>
            {resolvedNoDataMessage ?? 'No data available'}
          </span>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};

export default MFEDataWrapper;
