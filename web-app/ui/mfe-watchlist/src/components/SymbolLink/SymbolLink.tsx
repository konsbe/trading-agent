import { StockDetailLink } from '@trading-agent/shared-components';
import { useCanOpenStockDetail, useStockDetailOrigin } from '@/providers/StockDetailOrigin';
import { SymbolLinkProps } from './types';
import './SymbolLink-styles.css';

/**
 * A ticker: a link to its Stock Detail page (recording this screen, with its
 * sort and search, as "Back to …") for stocks and funds when hosted; plain
 * text for crypto, yields and anything standalone.
 */
const SymbolLink = ({ symbol, assetType, className = '', 'data-testid': testId }: SymbolLinkProps) => {
    const canOpen = useCanOpenStockDetail();
    const originLabel = useStockDetailOrigin();
    const classes = `symbol-link ${className}`.trim();

    return canOpen({ symbol, asset_type: assetType }) ? (
        <StockDetailLink className={`${classes} is-link`} symbol={symbol} originLabel={originLabel} data-testid={testId} />
    ) : (
        <span className={classes} data-testid={testId}>
            {symbol}
        </span>
    );
};

export default SymbolLink;
