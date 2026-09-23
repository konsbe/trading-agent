import React, { CSSProperties } from 'react';
import './Skeleton-styles.css';

export interface SkeletonProps {
    width?: CSSProperties['width'];
    height?: CSSProperties['height'];
    radius?: CSSProperties['borderRadius'];
    className?: string;
    'data-testid'?: string;
}

const Skeleton = ({
    width = '100%',
    height = '1rem',
    radius,
    className = '',
    'data-testid': testId = 'ta-skeleton',
}: SkeletonProps) => (
    <span
        aria-hidden="true"
        className={`ta-skeleton ${className}`.trim()}
        style={{ width, height, borderRadius: radius }}
        data-testid={testId}
    />
);

export default Skeleton;
