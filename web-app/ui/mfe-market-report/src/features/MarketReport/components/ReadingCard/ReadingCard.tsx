import { useId, useState } from 'react';
import { Button, ChevronDownIcon } from '@trading-agent/shared-components';
import { formatDate, formatNumber } from '../../utils/format';
import ToneIndicator from '../ToneIndicator';
import { ReadingCardProps } from './types';
import '@/styles/market-report-global.css';
import './ReadingCard-styles.css';

/**
 * One macro reading (a classification, or the market-cycle composite): the
 * stored label verbatim and the score, plus an optional in-card disclosure for
 * the details. A `classified` reading also shows the indicator for its stored
 * tone — and the no-data one when the section is missing.
 */
const ReadingCard = ({
    title,
    label,
    tone,
    classified = false,
    score,
    asOf,
    description,
    detailsLabel,
    children,
    unavailable = false,
    'data-testid': testId,
}: ReadingCardProps) => {
    const [open, setOpen] = useState(false);
    const detailsId = useId();

    return (
        <article className="market-report-tile market-report-reading" aria-label={title} data-testid={testId}>
            <h3 className="market-report-tile__title">{title}</h3>
            {unavailable ? (
                <p className="market-report-reading__headline" data-testid={testId && `${testId}-unavailable`}>
                    {classified && <ToneIndicator tone="no_data" data-testid={testId && `${testId}-tone`} />}
                    <span className="market-report-reading__unavailable">no data</span>
                </p>
            ) : (
                <>
                    <p className="market-report-reading__headline">
                        {classified && <ToneIndicator tone={tone} data-testid={testId && `${testId}-tone`} />}
                        <span className="market-report-reading__label" data-testid={testId && `${testId}-label`}>
                            {label ?? '—'}
                        </span>
                    </p>
                    <p className="market-report-muted market-report-small">
                        Score <span className="market-report-mono">{formatNumber(score)}</span>
                        {asOf && <> · as of {formatDate(asOf)}</>}
                    </p>
                    {description && <p className="market-report-reading__description">{description}</p>}
                    {children && detailsLabel && (
                        <>
                            <Button
                                variant="ghost"
                                size="sm"
                                className={`market-report-reading__toggle${open ? ' is-open' : ''}`}
                                aria-expanded={open}
                                aria-controls={detailsId}
                                onClick={() => setOpen(v => !v)}
                                data-testid={testId && `${testId}-toggle`}
                            >
                                {detailsLabel}
                                <ChevronDownIcon size={14} className="market-report-reading__chevron" />
                            </Button>
                            <div id={detailsId} hidden={!open} className="market-report-reading__details">
                                {open ? children : null}
                            </div>
                        </>
                    )}
                </>
            )}
        </article>
    );
};

export default ReadingCard;
