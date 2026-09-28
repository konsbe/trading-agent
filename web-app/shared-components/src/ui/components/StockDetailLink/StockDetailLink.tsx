import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { stockDetailLink } from './stockDetail';
import { StockDetailLinkProps } from './types';

/**
 * Router link to a symbol's Stock Detail page that records the current
 * location (pathname + search) as "Back to {originLabel}". Renders the symbol
 * unless children are given.
 */
const StockDetailLink = ({ symbol, originLabel, hash, children, ...rest }: StockDetailLinkProps) => {
    const { pathname, search } = useLocation();
    const target = stockDetailLink(symbol, { label: originLabel, from: `${pathname}${search}` }, { hash });
    return (
        <Link to={{ pathname: target.pathname, hash: target.hash }} state={target.state} {...rest}>
            {children ?? symbol}
        </Link>
    );
};

export default StockDetailLink;
