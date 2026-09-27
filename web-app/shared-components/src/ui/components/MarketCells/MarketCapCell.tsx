import React from 'react';
import { marketCapIsEstimate, marketCapText } from '../../../format';
import { MarketCapCellProps } from './types';
import './MarketCells-styles.css';

/** Reported cap, or the estimate with its "(est.)" marker (spec §2.2: never shown as if reported). */
const MarketCapCell = ({ row }: MarketCapCellProps) => {
    const estimate = marketCapIsEstimate(row);
    return (
        <span
            className={`market-cell__market-cap${estimate ? ' is-estimate' : ''}`}
            title={estimate ? 'Estimated: shares outstanding × close' : undefined}
            data-testid="market-cap-value"
        >
            {marketCapText(row)}
        </span>
    );
};

export default MarketCapCell;
