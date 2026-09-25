import { useId, useState } from 'react';
import { Button, ChevronDownIcon } from '@trading-agent/shared-components';
import { formatDate, formatNumber } from '../../utils/format';
import { ReadingCardProps } from './types';
import '@/styles/market-report-global.css';
import './ReadingCard-styles.css';

/**
 * One macro reading (a stance, the correlations regime, the market-cycle
 * composite): its plain-text label and score, plus an optional in-card
 * disclosure for the details. The label is the pipeline's own word — no colour.
 */
const ReadingCard = ({
    title,
    label,
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
                <p className="market-report-reading__unavailable" data-testid={testId && `${testId}-unavailable`}>
                    Unavailable
                </p>
            ) : (
                <>
                    <p className="market-report-reading__label" data-testid={testId && `${testId}-label`}>
                        {label ?? '—'}
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
