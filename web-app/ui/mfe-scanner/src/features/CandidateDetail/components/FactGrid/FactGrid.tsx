import { FactGridProps } from './types';
import './FactGrid-styles.css';

/** The Stock Detail fact grid (Primary facts and the analysis sections): label, value, optional sub line. */
const FactGrid = ({ cells, testIdPrefix = 'fact' }: FactGridProps) => (
    <dl className="scanner-facts__grid">
        {cells.map(cell => (
            <div key={cell.key} className={`scanner-facts__cell${cell.wide ? ' is-wide' : ''}`} data-testid={`${testIdPrefix}-${cell.key}`}>
                <dt className="scanner-facts__label">{cell.label}</dt>
                <dd className="scanner-facts__value-wrap">
                    <span className="scanner-facts__reading">
                        {cell.indicator}
                        <span
                            className={`scanner-facts__value${cell.plain ? ' is-plain' : ''}${cell.tone ? ` is-${cell.tone}` : ''}`}
                            data-testid={`${testIdPrefix}-${cell.key}-value`}
                        >
                            {cell.value}
                        </span>
                    </span>
                    {cell.sub && <span className="scanner-facts__sub">{cell.sub}</span>}
                </dd>
            </div>
        ))}
    </dl>
);

export default FactGrid;
