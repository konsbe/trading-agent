import { createSlice, PayloadAction } from '@reduxjs/toolkit';
import dayjs from 'dayjs';

export interface FilterData {
    products?: Array<Product>;
    timeframe?: TimeFrame;
}
export interface Product {
    name: string;
    cluster: string;
    elements: {name: string, version: string}[];
}
export interface TimeFrame {
    type: 'RELATIVE' | 'ABSOLUTE';
    from: number | string;
    to: number | string;
}

const initialState: FilterData = {
    products: [],
    timeframe: {
        type: 'RELATIVE',
        from: "Last 6 hr",
        to: "now",
    }
};

export const filterDataSlice = createSlice({
    name: 'filterData',
    initialState,
    reducers: {
        updateFilterData: (state, action: PayloadAction<FilterData>) => {
            return {
                timeframe: action.payload.timeframe || state.timeframe,
                products: action.payload.products || state.products
            };
        },
    },
});

export const { updateFilterData } = filterDataSlice.actions;
export default filterDataSlice.reducer;
