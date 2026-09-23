import { describe, it, expect } from '@jest/globals';
import {
    selectFilterDataState,
    selectFilterDataProducts,
    selectFilterDataTimeframe,
    selectFilterData,
    selectFilterDataProductsMemoized,
    selectFilterDataTimeframeMemoized,
    selectFilterDataInfo,
} from './filterDataSelectors';
import type { RootState } from '../../store/dataStore';
import { FilterData, Product, TimeFrame } from '../../slices/filterData/filterDataSlice';

describe('filterDataSelectors', () => {
    const mockProducts: Array<Product> = [
        {
            name: 'Product A',
            elements: [{name: 'element1', version: '1.0'}, {name: 'element2', version: '1.0'}],
        },
        {
            name: 'Product B',
            elements: [{name: 'element3', version: '1.0'}, {name: 'element4', version: '1.0'}],
        },
    ];

    const mockTimeframe: TimeFrame = {
        type: 'RELATIVE',
        from: 1234567890,
        to: 1234567900,
    };

    const mockState: RootState = {
        filterData: {
            products: mockProducts,
            timeframe: mockTimeframe,
        },
        user: {
            currentUser: {},
            isAuthenticated: false,
        },
    };

    describe('Basic selectors', () => {
        describe('selectFilterDataState', () => {
            it('should select the complete filterData state', () => {
                const result = selectFilterDataState(mockState);
                
                expect(result).toEqual(mockState.filterData);
                expect(result.products).toEqual(mockProducts);
                expect(result.timeframe).toEqual(mockTimeframe);
            });

            it('should return filterData when it is empty', () => {
                const emptyState: RootState = {
                    ...mockState,
                    filterData: {} as FilterData,
                };
                
                const result = selectFilterDataState(emptyState);
                
                expect(result).toEqual({});
            });

            it('should return filterData with only products', () => {
                const partialState: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                    } as FilterData,
                };
                
                const result = selectFilterDataState(partialState);
                
                expect(result.products).toEqual(mockProducts);
                expect(result.timeframe).toBeUndefined();
            });

            it('should return filterData with only timeframe', () => {
                const partialState: RootState = {
                    ...mockState,
                    filterData: {
                        timeframe: mockTimeframe,
                    } as FilterData,
                };
                
                const result = selectFilterDataState(partialState);
                
                expect(result.timeframe).toEqual(mockTimeframe);
                expect(result.products).toBeUndefined();
            });
        });

        describe('selectFilterDataProducts', () => {
            it('should select products from filterData', () => {
                const result = selectFilterDataProducts(mockState);
                
                expect(result).toEqual(mockProducts);
                expect(result).toHaveLength(2);
            });

            it('should return undefined when products is not defined', () => {
                const stateWithoutProducts: RootState = {
                    ...mockState,
                    filterData: {
                        timeframe: mockTimeframe,
                    } as FilterData,
                };
                
                const result = selectFilterDataProducts(stateWithoutProducts);
                
                expect(result).toBeUndefined();
            });

            it('should return empty array when products is empty', () => {
                const stateWithEmptyProducts: RootState = {
                    ...mockState,
                    filterData: {
                        products: [],
                        timeframe: mockTimeframe,
                    },
                };
                
                const result = selectFilterDataProducts(stateWithEmptyProducts);
                
                expect(result).toEqual([]);
                expect(result).toHaveLength(0);
            });

            it('should return single product in array', () => {
                const stateWithOneProduct: RootState = {
                    ...mockState,
                    filterData: {
                        products: [mockProducts[0]],
                        timeframe: mockTimeframe,
                    },
                };
                
                const result = selectFilterDataProducts(stateWithOneProduct);
                
                expect(result).toHaveLength(1);
                expect(result?.[0]).toEqual(mockProducts[0]);
            });
        });

        describe('selectFilterDataTimeframe', () => {
            it('should select timeframe from filterData', () => {
                const result = selectFilterDataTimeframe(mockState);
                
                expect(result).toEqual(mockTimeframe);
                expect(result?.type).toBe('RELATIVE');
                expect(result?.from).toBe(1234567890);
                expect(result?.to).toBe(1234567900);
            });

            it('should return undefined when timeframe is not defined', () => {
                const stateWithoutTimeframe: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                    } as FilterData,
                };
                
                const result = selectFilterDataTimeframe(stateWithoutTimeframe);
                
                expect(result).toBeUndefined();
            });

            it('should select ABSOLUTE timeframe type', () => {
                const absoluteTimeframe: TimeFrame = {
                    type: 'ABSOLUTE',
                    from: '2026-01-01T00:00:00Z',
                    to: '2026-01-12T23:59:59Z',
                };
                const stateWithAbsolute: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                        timeframe: absoluteTimeframe,
                    },
                };
                
                const result = selectFilterDataTimeframe(stateWithAbsolute);
                
                expect(result).toEqual(absoluteTimeframe);
                expect(result?.type).toBe('ABSOLUTE');
            });

            it('should handle timeframe with zero values', () => {
                const zeroTimeframe: TimeFrame = {
                    type: 'RELATIVE',
                    from: 0,
                    to: 0,
                };
                const stateWithZero: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                        timeframe: zeroTimeframe,
                    },
                };
                
                const result = selectFilterDataTimeframe(stateWithZero);
                
                expect(result?.from).toBe(0);
                expect(result?.to).toBe(0);
            });
        });
    });

    describe('Memoized selectors', () => {
        describe('selectFilterData', () => {
            it('should return the complete filterData using memoization', () => {
                const result = selectFilterData(mockState);
                
                expect(result).toEqual(mockState.filterData);
            });

            it('should return the same reference for identical state', () => {
                const result1 = selectFilterData(mockState);
                const result2 = selectFilterData(mockState);
                
                expect(result1).toBe(result2);
            });

            it('should return different reference when state changes', () => {
                const result1 = selectFilterData(mockState);
                
                const modifiedState: RootState = {
                    ...mockState,
                    filterData: {
                        products: [...mockProducts, { name: 'Product C', elements: ['element5'] }],
                        timeframe: mockTimeframe,
                    },
                };
                
                const result2 = selectFilterData(modifiedState);
                
                expect(result1).not.toBe(result2);
            });

            it('should handle empty filterData', () => {
                const emptyState: RootState = {
                    ...mockState,
                    filterData: {} as FilterData,
                };
                
                const result = selectFilterData(emptyState);
                
                expect(result).toEqual({});
            });
        });

        describe('selectFilterDataProductsMemoized', () => {
            it('should return products array', () => {
                const result = selectFilterDataProductsMemoized(mockState);
                
                expect(result).toEqual(mockProducts);
            });

            it('should return empty array when products is undefined', () => {
                const stateWithoutProducts: RootState = {
                    ...mockState,
                    filterData: {
                        timeframe: mockTimeframe,
                    } as FilterData,
                };
                
                const result = selectFilterDataProductsMemoized(stateWithoutProducts);
                
                expect(result).toEqual([]);
            });

            it('should return empty array when products is null', () => {
                const stateWithNullProducts: RootState = {
                    ...mockState,
                    filterData: {
                        products: null as any,
                        timeframe: mockTimeframe,
                    },
                };
                
                const result = selectFilterDataProductsMemoized(stateWithNullProducts);
                
                expect(result).toEqual([]);
            });

            it('should return the same reference for identical state', () => {
                const result1 = selectFilterDataProductsMemoized(mockState);
                const result2 = selectFilterDataProductsMemoized(mockState);
                
                expect(result1).toBe(result2);
            });

            it('should return different reference when products change', () => {
                const result1 = selectFilterDataProductsMemoized(mockState);
                
                const modifiedState: RootState = {
                    ...mockState,
                    filterData: {
                        products: [...mockProducts, { name: 'Product C', elements: [] }],
                        timeframe: mockTimeframe,
                    },
                };
                
                const result2 = selectFilterDataProductsMemoized(modifiedState);
                
                expect(result1).not.toBe(result2);
            });

            it('should handle empty products array', () => {
                const stateWithEmptyProducts: RootState = {
                    ...mockState,
                    filterData: {
                        products: [],
                        timeframe: mockTimeframe,
                    },
                };
                
                const result = selectFilterDataProductsMemoized(stateWithEmptyProducts);
                
                expect(result).toEqual([]);
            });
        });

        describe('selectFilterDataTimeframeMemoized', () => {
            it('should return timeframe object', () => {
                const result = selectFilterDataTimeframeMemoized(mockState);
                
                expect(result).toEqual(mockTimeframe);
            });

            it('should return null when timeframe is undefined', () => {
                const stateWithoutTimeframe: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                    } as FilterData,
                };
                
                const result = selectFilterDataTimeframeMemoized(stateWithoutTimeframe);
                
                expect(result).toBeNull();
            });

            it('should return null when timeframe is null', () => {
                const stateWithNullTimeframe: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                        timeframe: null as any,
                    },
                };
                
                const result = selectFilterDataTimeframeMemoized(stateWithNullTimeframe);
                
                expect(result).toBeNull();
            });

            it('should return the same reference for identical state', () => {
                const result1 = selectFilterDataTimeframeMemoized(mockState);
                const result2 = selectFilterDataTimeframeMemoized(mockState);
                
                expect(result1).toBe(result2);
            });

            it('should return different reference when timeframe changes', () => {
                const result1 = selectFilterDataTimeframeMemoized(mockState);
                
                const modifiedState: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                        timeframe: {
                            type: 'ABSOLUTE',
                            from: '2026-01-01T00:00:00Z',
                            to: '2026-01-12T23:59:59Z',
                        },
                    },
                };
                
                const result2 = selectFilterDataTimeframeMemoized(modifiedState);
                
                expect(result1).not.toBe(result2);
            });

            it('should handle timeframe with zero values', () => {
                const stateWithZeroTimeframe: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                        timeframe: {
                            type: 'RELATIVE',
                            from: 0,
                            to: 0,
                        },
                    },
                };
                
                const result = selectFilterDataTimeframeMemoized(stateWithZeroTimeframe);
                
                expect(result).toEqual({
                    type: 'RELATIVE',
                    from: 0,
                    to: 0,
                });
            });
        });
    });

    describe('Combined selectors', () => {
        describe('selectFilterDataInfo', () => {
            it('should return combined products and timeframe', () => {
                const result = selectFilterDataInfo(mockState);
                
                expect(result).toEqual({
                    products: mockProducts,
                    timeframe: mockTimeframe,
                });
            });

            it('should return empty array for products and null for timeframe when not defined', () => {
                const emptyState: RootState = {
                    ...mockState,
                    filterData: {} as FilterData,
                };
                
                const result = selectFilterDataInfo(emptyState);
                
                expect(result.products).toEqual([]);
                expect(result.timeframe).toBeNull();
            });

            it('should return products with null timeframe', () => {
                const stateWithoutTimeframe: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                    } as FilterData,
                };
                
                const result = selectFilterDataInfo(stateWithoutTimeframe);
                
                expect(result.products).toEqual(mockProducts);
                expect(result.timeframe).toBeNull();
            });

            it('should return empty products with timeframe', () => {
                const stateWithoutProducts: RootState = {
                    ...mockState,
                    filterData: {
                        timeframe: mockTimeframe,
                    } as FilterData,
                };
                
                const result = selectFilterDataInfo(stateWithoutProducts);
                
                expect(result.products).toEqual([]);
                expect(result.timeframe).toEqual(mockTimeframe);
            });

            it('should return the same reference for identical state', () => {
                const result1 = selectFilterDataInfo(mockState);
                const result2 = selectFilterDataInfo(mockState);
                
                expect(result1).toBe(result2);
            });

            it('should return different reference when products change', () => {
                const result1 = selectFilterDataInfo(mockState);
                
                const modifiedState: RootState = {
                    ...mockState,
                    filterData: {
                        products: [...mockProducts, { name: 'Product C', elements: ['element5'] }],
                        timeframe: mockTimeframe,
                    },
                };
                
                const result2 = selectFilterDataInfo(modifiedState);
                
                expect(result1).not.toBe(result2);
                expect(result2.products).toHaveLength(3);
            });

            it('should return different reference when timeframe changes', () => {
                const result1 = selectFilterDataInfo(mockState);
                
                const modifiedState: RootState = {
                    ...mockState,
                    filterData: {
                        products: mockProducts,
                        timeframe: {
                            type: 'ABSOLUTE',
                            from: '2026-01-01T00:00:00Z',
                            to: '2026-01-12T23:59:59Z',
                        },
                    },
                };
                
                const result2 = selectFilterDataInfo(modifiedState);
                
                expect(result1).not.toBe(result2);
                expect(result2.timeframe?.type).toBe('ABSOLUTE');
            });

            it('should handle both null products and timeframe', () => {
                const stateWithNulls: RootState = {
                    ...mockState,
                    filterData: {
                        products: null as any,
                        timeframe: null as any,
                    },
                };
                
                const result = selectFilterDataInfo(stateWithNulls);
                
                expect(result.products).toEqual([]);
                expect(result.timeframe).toBeNull();
            });

            it('should maintain structure consistency with different data', () => {
                const differentState: RootState = {
                    ...mockState,
                    filterData: {
                        products: [{ name: 'Product X', elements: ['x1', 'x2', 'x3'] }],
                        timeframe: {
                            type: 'ABSOLUTE',
                            from: '2025-01-01T00:00:00Z',
                            to: '2025-12-31T23:59:59Z',
                        },
                    },
                };
                
                const result = selectFilterDataInfo(differentState);
                
                expect(result).toHaveProperty('products');
                expect(result).toHaveProperty('timeframe');
                expect(result.products).toHaveLength(1);
                expect(result.timeframe?.type).toBe('ABSOLUTE');
            });
        });
    });

    describe('Memoization behavior', () => {
        it('should not recompute when unrelated state changes', () => {
            const result1 = selectFilterDataInfo(mockState);
            
            const stateWithUserChange: RootState = {
                ...mockState,
                user: {
                    currentUser: { userName: 'Test User' },
                    isAuthenticated: true,
                },
            };
            
            const result2 = selectFilterDataInfo(stateWithUserChange);
            
            // Should be the same reference due to memoization
            expect(result1).toBe(result2);
        });

        it('should recompute when filterData changes', () => {
            const result1 = selectFilterDataInfo(mockState);
            
            const modifiedState: RootState = {
                ...mockState,
                filterData: {
                    products: [],
                    timeframe: mockTimeframe,
                },
            };
            
            const result2 = selectFilterDataInfo(modifiedState);
            
            expect(result1).not.toBe(result2);
        });
    });
});
