import React from 'react';
import { ContentRenderer } from '../ContentRenderer/ContentRenderer';
import { type ContentWrapperProps } from '../../types/content-wrapper';
import ErrorBoundary from '../ErrorBoundary/ErrorBoundary';
import FullSizeSkeleton from '../Skeleton/FullSizeSkeleton';


export const ContentWrapper = (props: ContentWrapperProps) => {
    const {
        id,
        className = '',
        style = { height: '100%' },
        loadingComponent = <FullSizeSkeleton />,
    } = props;

    return (
        <ErrorBoundary>
            <div
                id={id}
                className={className}
                style={style}
            >
                <ContentRenderer
                    props={props}
                    loadingComponent={loadingComponent}
                />
            </div>
        </ErrorBoundary>
    );
};
