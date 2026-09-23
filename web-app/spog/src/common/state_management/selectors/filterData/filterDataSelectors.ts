import { createSelector } from '@reduxjs/toolkit';
import type { RootState } from '../../store/dataStore';

// Basic selectors
export const selectFilterDataState = (state: RootState) => state.filterData;
export const selectFilterDataProducts = (state: RootState) => state.filterData.products;
export const selectFilterDataTimeframe = (state: RootState) => state.filterData.timeframe;

// Memoized selectors using createSelector
export const selectFilterData = createSelector(
    [selectFilterDataState],
    (filterData) => filterData
);

export const selectFilterDataProductsMemoized = createSelector(
    [selectFilterDataProducts],
    (products) => products || []
);

export const selectFilterDataTimeframeMemoized = createSelector(
    [selectFilterDataTimeframe],
    (timeframe) => timeframe || null
);

// Combined selectors
export const selectFilterDataInfo = createSelector(
    [selectFilterDataProductsMemoized, selectFilterDataTimeframeMemoized],
    (products, timeframe) => ({
        products,
        timeframe,
    })
);
