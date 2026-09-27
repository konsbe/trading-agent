import React from 'react';
import { formatSignedPercent } from '../../../format';
import { ChangeCellProps } from './types';
import './MarketCells-styles.css';

/** The only toned market value: a price delta, so --color-price-up/down (zero and null stay neutral). */
const ChangeCell = ({ value }: ChangeCellProps) => {
    const tone = value === null || !Number.isFinite(value) || value === 0 ? '' : value > 0 ? ' is-price-up' : ' is-price-down';
    return (
        <span className={`market-cell__change${tone}`} data-testid="change-value">
            {formatSignedPercent(value)}
        </span>
    );
};

export default ChangeCell;
