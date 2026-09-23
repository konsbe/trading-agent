import { describe, it, expect } from '@jest/globals';
import { Provider } from 'react-redux';
import configureStore from 'redux-mock-store';
import { renderHook } from '@testing-library/react';
import { useFilterData, useFilterDataProducts, useFilterDataTimeframe } from './useFilterData';
import { FilterData, Product, TimeFrame } from '../../slices/filterData/filterDataSlice';

const mockStore = configureStore([]);

describe('useFilterData', () => {
    const mockProducts: Array<Product> = [
        {
            name: 'Product A',
            elements: [{name: 'element1', version: '1.0'}, {name: 'element2', version: '1.0'}],
        },
        {
            name: 'Product B',
            elements: [{name: 'element3', version: '1.0'}, {name: 'element4', version: '1.0'}, {name: 'element5', version: '1.0'}],
        },
    ];

    const mockTimeframe: TimeFrame = {
        type: 'RELATIVE',
        from: 1234567890,
        to: 1234567900,
    };

    const initialState = {
        filterData: {
            products: mockProducts,
            timeframe: mockTimeframe,
        } as FilterData,
    };

    describe('useFilterData', () => {
        it('should return complete filter data from store', () => {
            const store = mockStore(initialState);

            const { result } = renderHook(() => useFilterData(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual(initialState.filterData);
            expect(result.current?.products).toEqual(mockProducts);
            expect(result.current?.timeframe).toEqual(mockTimeframe);
        });

        it('should return null when filter data is null', () => {
            const emptyState = {
                filterData: null,
            };
            const store = mockStore(emptyState);

            const { result } = renderHook(() => useFilterData(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toBeNull();
        });

        it('should return filter data with undefined fields when they are missing', () => {
            const partialState = {
                filterData: {} as FilterData,
            };
            const store = mockStore(partialState);

            const { result } = renderHook(() => useFilterData(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual({});
            expect(result.current?.products).toBeUndefined();
            expect(result.current?.timeframe).toBeUndefined();
        });
    });

    describe('useFilterDataProducts', () => {
        it('should return products array from filter data', () => {
            const store = mockStore(initialState);

            const { result } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual(mockProducts);
            expect(result.current).toHaveLength(2);
            expect(result.current[0].name).toBe('Product A');
            expect(result.current[0].elements).toEqual([{name: 'element1', version: '1.0'}, {name: 'element2', version: '1.0'}]);
            expect(result.current[1].name).toBe('Product B');
            expect(result.current[1].elements).toEqual([{name: 'element3', version: '1.0'}, {name: 'element4', version: '1.0'}, {name: 'element5', version: '1.0'}]);
        });

        it('should return empty array when products is undefined', () => {
            const stateWithoutProducts = {
                filterData: {
                    timeframe: mockTimeframe,
                } as FilterData,
            };
            const store = mockStore(stateWithoutProducts);

            const { result } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual([]);
            expect(result.current).toHaveLength(0);
        });

        it('should return empty array when products is null', () => {
            const stateWithNullProducts = {
                filterData: {
                    products: null,
                    timeframe: mockTimeframe,
                } as FilterData,
            };
            const store = mockStore(stateWithNullProducts);

            const { result } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual([]);
        });

        it('should return empty array when filterData is null', () => {
            const emptyState = {
                filterData: null,
            };
            const store = mockStore(emptyState);

            const { result } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual([]);
        });

        it('should return empty array when products array is empty', () => {
            const stateWithEmptyProducts = {
                filterData: {
                    products: [],
                    timeframe: mockTimeframe,
                } as FilterData,
            };
            const store = mockStore(stateWithEmptyProducts);

            const { result } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual([]);
            expect(result.current).toHaveLength(0);
        });

        it('should return single product when products array has one element', () => {
            const stateWithOneProduct = {
                filterData: {
                    products: [mockProducts[0]],
                    timeframe: mockTimeframe,
                } as FilterData,
            };
            const store = mockStore(stateWithOneProduct);

            const { result } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toHaveLength(1);
            expect(result.current[0]).toEqual(mockProducts[0]);
        });
    });

    describe('useFilterDataTimeframe', () => {
        it('should return timeframe from filter data with RELATIVE type', () => {
            const store = mockStore(initialState);

            const { result } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual(mockTimeframe);
            expect(result.current?.type).toBe('RELATIVE');
            expect(result.current?.from).toBe(1234567890);
            expect(result.current?.to).toBe(1234567900);
        });

        it('should return timeframe with ABSOLUTE type and string dates', () => {
            const absoluteTimeframe: TimeFrame = {
                type: 'ABSOLUTE',
                from: '2026-01-01T00:00:00Z',
                to: '2026-01-12T23:59:59Z',
            };
            const stateWithAbsolute = {
                filterData: {
                    products: mockProducts,
                    timeframe: absoluteTimeframe,
                } as FilterData,
            };
            const store = mockStore(stateWithAbsolute);

            const { result } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual(absoluteTimeframe);
            expect(result.current?.type).toBe('ABSOLUTE');
            expect(result.current?.from).toBe('2026-01-01T00:00:00Z');
            expect(result.current?.to).toBe('2026-01-12T23:59:59Z');
        });

        it('should return null when timeframe is undefined', () => {
            const stateWithoutTimeframe = {
                filterData: {
                    products: mockProducts,
                } as FilterData,
            };
            const store = mockStore(stateWithoutTimeframe);

            const { result } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toBeNull();
        });

        it('should return null when timeframe is null', () => {
            const stateWithNullTimeframe = {
                filterData: {
                    products: mockProducts,
                    timeframe: null,
                } as FilterData,
            };
            const store = mockStore(stateWithNullTimeframe);

            const { result } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toBeNull();
        });

        it('should return null when filterData is null', () => {
            const emptyState = {
                filterData: null,
            };
            const store = mockStore(emptyState);

            const { result } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toBeNull();
        });

        it('should handle timeframe with zero values', () => {
            const zeroTimeframe: TimeFrame = {
                type: 'RELATIVE',
                from: 0,
                to: 0,
            };
            const stateWithZeroTimeframe = {
                filterData: {
                    products: mockProducts,
                    timeframe: zeroTimeframe,
                } as FilterData,
            };
            const store = mockStore(stateWithZeroTimeframe);

            const { result } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(result.current).toEqual(zeroTimeframe);
            expect(result.current?.from).toBe(0);
            expect(result.current?.to).toBe(0);
        });
    });

    describe('Integration tests', () => {
        it('should return consistent data across all hooks', () => {
            const store = mockStore(initialState);

            const { result: filterDataResult } = renderHook(() => useFilterData(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            const { result: productsResult } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            const { result: timeframeResult } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(filterDataResult.current?.products).toEqual(productsResult.current);
            expect(filterDataResult.current?.timeframe).toEqual(timeframeResult.current);
        });

        it('should handle empty state consistently across all hooks', () => {
            const emptyState = {
                filterData: null,
            };
            const store = mockStore(emptyState);

            const { result: filterDataResult } = renderHook(() => useFilterData(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            const { result: productsResult } = renderHook(() => useFilterDataProducts(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            const { result: timeframeResult } = renderHook(() => useFilterDataTimeframe(), {
                wrapper: ({ children }) => <Provider store={store}>{children}</Provider>,
            });

            expect(filterDataResult.current).toBeNull();
            expect(productsResult.current).toEqual([]);
            expect(timeframeResult.current).toBeNull();
        });
    });
});
