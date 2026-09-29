import { useState } from 'react';
import { Button, CollapsibleCard, TableSearch, useTableView } from '@trading-agent/shared-components';
import { BUCKET_LABELS, DEFAULT_SORT, WINDOW_SIZE } from '../../constants';
import CandidatesTable, { COLUMNS, columnLabel } from '../CandidatesTable';
import { BucketSectionProps } from './types';
import './BucketSection-styles.css';

/**
 * One bucket in a collapsible card: a searchable, sortable table windowed to
 * the top WINDOW_SIZE rows of the current sort, with "and N more" revealing
 * the rest. Sort and search live in the URL (`<bucket>_sort`, `<bucket>_q`)
 * and windowing here, outside the collapsible content, so they survive
 * collapse/expand and "Back to Candidates" from Stock Detail.
 */
const BucketSection = ({ bucket, result }: BucketSectionProps) => {
    const view = useTableView({ rows: result.candidates, columns: COLUMNS, defaultSort: DEFAULT_SORT, urlKey: bucket });
    const [showAll, setShowAll] = useState(false);

    const label = BUCKET_LABELS[bucket];
    const tableId = `scanner-bucket-${bucket}-table`;
    const hiddenCount = Math.max(view.shown - WINDOW_SIZE, 0);
    const visible = showAll ? view.rows : view.rows.slice(0, WINDOW_SIZE);

    return (
        <CollapsibleCard
            id={`scanner-bucket-${bucket}`}
            persistKey={`scanner.list.${bucket}`}
            fit
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
                view.total > 0 ? (
                    <p className="scanner-bucket__sort-label" aria-live="polite" data-testid={`bucket-${bucket}-sort-label`}>
                        Sorted by {columnLabel(view.sort.key)} — descriptive, not predictive
                    </p>
                ) : undefined
            }
        >
            {view.total === 0 ? (
                <p className="scanner-bucket__empty" data-testid={`bucket-${bucket}-empty`}>
                    No {label} candidates in this scan
                </p>
            ) : (
                <>
                    <TableSearch
                        label={`Search ${label} candidates`}
                        value={view.query}
                        onChange={view.setQuery}
                        total={view.total}
                        shown={view.shown}
                        controls={tableId}
                        data-testid={`bucket-${bucket}-search`}
                    />
                    {view.shown === 0 ? (
                        <p className="scanner-bucket__empty" data-testid={`bucket-${bucket}-no-match`}>
                            No {label} candidates match “{view.query.trim()}”
                        </p>
                    ) : (
                        <CandidatesTable id={tableId} caption={`${label} candidates`} rows={visible} headerProps={view.headerProps} />
                    )}
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
