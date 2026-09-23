import React from 'react';
import Skeleton from './Skeleton';
import './FullSizeSkeleton-styles.css';

const FullSizeSkeleton = () => (
    <div data-testid="full-size-skeleton-wrapper" className="full-size-skeleton-wrapper">
        <Skeleton width="100%" height="100%" radius="var(--radius-md)" />
    </div>
);

export default FullSizeSkeleton;
