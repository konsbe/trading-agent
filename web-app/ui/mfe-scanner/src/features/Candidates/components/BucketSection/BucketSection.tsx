import { useState } from 'react';
import { Button, CollapsibleCard } from '@trading-agent/shared-components';
import { BUCKET_LABELS, WINDOW_SIZE } from '../../constants';
import useCandidateSort from '../../hooks/useCandidateSort';
import CandidatesTable, { columnLabel } from '../CandidatesTable';
import { BucketSectionProps } from './types';
import './BucketSection-styles.css';

/**
 * One bucket in a collapsible card: sortable table windowed to the top
 * WINDOW_SIZE rows of the current sort, with "and N more" revealing the rest.
 * Sort and windowing state live here, outside the collapsible content, so
 * they survive collapse/expand.
 */
const BucketSection = ({ bucket, result }: BucketSectionProps) => {
    const { sort, sorted, toggleSort } = useCandidateSort(result.candidates);
    const [showAll, setShowAll] = useState(false);

    const label = BUCKET_LABELS[bucket];
    const tableId = `scanner-bucket-${bucket}-table`;
    const hiddenCount = Math.max(sorted.length - WINDOW_SIZE, 0);
    const visible = showAll ? sorted : sorted.slice(0, WINDOW_SIZE);

    return (
        <CollapsibleCard
            id={`scanner-bucket-${bucket}`}
            persistKey={`scanner.list.${bucket}`}
            className="scanner-bucket"
            data-testid={`bucket-${bucket}`}
            title={
                <span className="scanner-bucket__title">
                    {label}
                    <span
                        className="scanner-bucket__count"
                        data-testid={`bucket-${bucket}-count`}
                        aria-label={`${result.total_candidates} candidates`}
                    >
                        {result.total_candidates}
                    </span>
                </span>
            }
            meta={
                sorted.length > 0 ? (
                    <p className="scanner-bucket__sort-label" aria-live="polite" data-testid={`bucket-${bucket}-sort-label`}>
                        Sorted by {columnLabel(sort.key)} — descriptive, not predictive
                    </p>
                ) : undefined
            }
        >
            {sorted.length === 0 ? (
                <p className="scanner-bucket__empty" data-testid={`bucket-${bucket}-empty`}>
                    No {label} candidates in this scan
                </p>
            ) : (
                <>
                    <CandidatesTable
                        id={tableId}
                        caption={`${label} candidates`}
                        rows={visible}
                        sort={sort}
                        onSort={toggleSort}
                    />
                    {hiddenCount > 0 && (
                        <div className="scanner-bucket__more">
                            <Button
                                variant="ghost"
                                size="sm"
                                aria-expanded={showAll}
                                aria-controls={tableId}
                                onClick={() => setShowAll(value => !value)}
                                data-testid={`bucket-${bucket}-toggle`}
                            >
                                {showAll ? `Show top ${WINDOW_SIZE} only` : `and ${hiddenCount} more`}
                            </Button>
                        </div>
                    )}
                </>
            )}
        </CollapsibleCard>
    );
};

export default BucketSection;
