import { Skeleton } from '@trading-agent/shared-components';
import { DocSkeletonProps } from './types';
import './DocSkeleton-styles.css';

const LINE_WIDTHS = ['100%', '94%', '97%', '60%'];

/** Placeholder text lines while Education content loads. */
const DocSkeleton = ({ label }: DocSkeletonProps) => (
    <div className="education-skeleton" aria-busy="true" aria-label={label} role="status" data-testid="doc-skeleton">
        <Skeleton width="40%" height="1.5rem" radius="var(--radius-md)" />
        {[0, 1].map(block => (
            <div key={block} className="education-skeleton__block">
                {LINE_WIDTHS.map((width, index) => (
                    <Skeleton key={index} width={width} height="0.875rem" radius="var(--radius-sm)" />
                ))}
            </div>
        ))}
    </div>
);

export default DocSkeleton;
