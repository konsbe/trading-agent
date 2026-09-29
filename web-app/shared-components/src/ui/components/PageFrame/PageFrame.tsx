import React from 'react';
import PageHeader from '../PageHeader';
import { PageFrameProps } from './types';
import './PageFrame-styles.css';

/**
 * An MFE page that fills its shell cell: the header bar (shared PageHeader,
 * `bar` variant) fixed at the top, then the body, which is the page's only
 * scroll container. A fit-to-height page puts `ta-fit` columns and one
 * `ta-fit-scroll` region in the body so that region scrolls instead.
 */
const PageFrame = ({
    title,
    subtitle,
    back,
    badges,
    actions,
    className = '',
    bodyClassName = '',
    'data-testid': testId,
    children,
}: PageFrameProps) => (
    <div className={`ta-page-frame ${className}`.trim()} data-testid={testId}>
        {title && <PageHeader variant="bar" title={title} subtitle={subtitle} back={back} badges={badges} actions={actions} />}
        <div className={`ta-page-frame__body ${bodyClassName}`.trim()} data-testid={testId ? `${testId}-body` : undefined}>
            {children}
        </div>
    </div>
);

export default PageFrame;
