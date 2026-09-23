import reducer, {
    updateFilterData,
    FilterData,
    Product,
    TimeFrame,
} from './filterDataSlice';
import dayjs from 'dayjs';

// Mock dayjs to have consistent test results
jest.mock('dayjs', () => {
    const mockDayjs = jest.fn(() => ({
        subtract: jest.fn(() => ({
            valueOf: jest.fn(() => 1000000000000),
        })),
        valueOf: jest.fn(() => 1000000600000),
    }));
    return mockDayjs;
});

describe('filterDataSlice', () => {
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

    const initialState: FilterData = {
        products: [],
        timeframe: {
            type: 'ABSOLUTE',
            from: 1000000000000,
            to: 1000000600000,
        }
    };

    describe('Initial State', () => {
        it('should return the initial state with empty products', () => {
            const state = reducer(undefined, { type: '' });
            
            expect(state.products).toEqual([]);
            expect(state.products).toHaveLength(0);
        });

        it('should return the initial state with timeframe from dayjs', () => {
            const state = reducer(undefined, { type: '' });
            
            expect(state.timeframe).toBeDefined();
            expect(state.timeframe?.type).toBe('RELATIVE');
            expect(state.timeframe?.from).toBe('Last 6 hr');
            expect(state.timeframe?.to).toBe('now');
        });

        it('should have correct structure', () => {
            const state = reducer(undefined, { type: '' });
            
            expect(state).toHaveProperty('products');
            expect(state).toHaveProperty('timeframe');
        });
    });

    describe('updateFilterData', () => {
        it('should update complete filter data', () => {
            const payload: FilterData = {
                products: mockProducts,
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState).toEqual(payload);
            expect(nextState.products).toEqual(mockProducts);
            expect(nextState.timeframe).toEqual(mockTimeframe);
        });

        it('should replace entire state with new data', () => {
            const currentState: FilterData = {
                products: [{ name: 'Old Product', elements: [{name: 'old1', version: '1.0'}] }],
                timeframe: {
                    type: 'ABSOLUTE',
                    from: 'old-date',
                    to: 'old-date',
                },
            };
            
            const payload: FilterData = {
                products: mockProducts,
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(currentState, updateFilterData(payload));
            
            expect(nextState).toEqual(payload);
            expect(nextState.products).not.toEqual(currentState.products);
        });

        it('should update with empty products array', () => {
            const payload: FilterData = {
                products: [],
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products).toEqual([]);
            expect(nextState.products).toHaveLength(0);
        });

        it('should update with undefined products', () => {
            const payload: FilterData = {
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products).toEqual([]);
            expect(nextState.timeframe).toEqual(mockTimeframe);
        });

        it('should update with undefined timeframe', () => {
            const payload: FilterData = {
                products: mockProducts,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products).toEqual(mockProducts);
            expect(nextState.timeframe).toEqual(initialState.timeframe);
        });

        it('should preserve existing timeframe when payload has null timeframe', () => {
            const currentState: FilterData = {
                products: [],
                timeframe: mockTimeframe,
            };
            
            const payload: FilterData = {
                products: mockProducts,
                timeframe: null as any, // Simulating MFE sending null
            };
            
            const nextState = reducer(currentState, updateFilterData(payload));
            
            expect(nextState.products).toEqual(mockProducts);
            expect(nextState.timeframe).toEqual(mockTimeframe); // Should preserve existing timeframe
        });

        it('should preserve existing products when payload has null products', () => {
            const currentState: FilterData = {
                products: mockProducts,
                timeframe: mockTimeframe,
            };
            
            const payload: FilterData = {
                products: null as any, // Simulating MFE sending null
                timeframe: {
                    type: 'ABSOLUTE',
                    from: 'new-date',
                    to: 'new-date',
                },
            };
            
            const nextState = reducer(currentState, updateFilterData(payload));
            
            expect(nextState.products).toEqual(mockProducts); // Should preserve existing products
            expect(nextState.timeframe).toEqual(payload.timeframe);
        });

        it('should preserve both when payload has null values', () => {
            const currentState: FilterData = {
                products: mockProducts,
                timeframe: mockTimeframe,
            };
            
            const payload: FilterData = {
                products: null as any,
                timeframe: null as any,
            };
            
            const nextState = reducer(currentState, updateFilterData(payload));
            
            expect(nextState.products).toEqual(mockProducts);
            expect(nextState.timeframe).toEqual(mockTimeframe);
        });

        it('should update with single product', () => {
            const payload: FilterData = {
                products: [mockProducts[0]],
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products).toHaveLength(1);
            expect(nextState.products?.[0]).toEqual(mockProducts[0]);
        });

        it('should update with ABSOLUTE timeframe type', () => {
            const absoluteTimeframe: TimeFrame = {
                type: 'ABSOLUTE',
                from: '2026-01-01T00:00:00Z',
                to: '2026-01-12T23:59:59Z',
            };
            const payload: FilterData = {
                products: mockProducts,
                timeframe: absoluteTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.timeframe?.type).toBe('ABSOLUTE');
            expect(nextState.timeframe?.from).toBe('2026-01-01T00:00:00Z');
            expect(nextState.timeframe?.to).toBe('2026-01-12T23:59:59Z');
        });

        it('should update with empty object', () => {
            const payload: FilterData = {};
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products).toEqual(initialState.products);
            expect(nextState.timeframe).toEqual(initialState.timeframe);
        });

        it('should handle multiple products with many elements', () => {
            const manyProducts: Array<Product> = [
                { name: 'Product 1', elements: [{name: 'e1', version: '1.0'}, {name: 'e2', version: '1.0'}, {name: 'e3', version: '1.0'}, {name: 'e4', version: '1.0'}, {name: 'e5', version: '1.0'}] },
                { name: 'Product 2', elements: [{name: 'e6', version: '1.0'}, {name: 'e7', version: '1.0'}, {name: 'e8', version: '1.0'}] },
                { name: 'Product 3', elements: [{name: 'e9', version: '1.0'}] },
            ];
            const payload: FilterData = {
                products: manyProducts,
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products).toHaveLength(3);
            expect(nextState.products?.[0].elements).toHaveLength(5);
        });
    });


    describe('Action creators', () => {
        it('should create updateFilterData action with correct payload', () => {
            const payload: FilterData = {
                products: mockProducts,
                timeframe: mockTimeframe,
            };
            
            const action = updateFilterData(payload);
            
            expect(action.type).toBe('filterData/updateFilterData');
            expect(action.payload).toEqual(payload);
        });


    });

    describe('Edge cases', () => {
        it('should handle products with empty elements array', () => {
            const productsWithEmptyElements: Array<Product> = [
                { name: 'Product X', elements: [] },
            ];
            const payload: FilterData = {
                products: productsWithEmptyElements,
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products?.[0].elements).toEqual([]);
            expect(nextState.products?.[0].elements).toHaveLength(0);
        });

        it('should handle products with special characters in names', () => {
            const specialProducts: Array<Product> = [
                { name: 'Product @#$%', elements: [{name: 'elem1', version: '1.0'}] },
                { name: 'Product-with-dashes', elements: [{name: 'elem2', version: '1.0'}] },
                { name: 'Product_with_underscores', elements: [{name: 'elem3', version: '1.0'}] },
            ];
            const payload: FilterData = {
                products: specialProducts,
                timeframe: mockTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.products).toHaveLength(3);
            expect(nextState.products?.[0].name).toBe('Product @#$%');
        });

        it('should handle very large timestamp values', () => {
            const largeTimeframe: TimeFrame = {
                type: 'RELATIVE',
                from: 9999999999999,
                to: 9999999999999,
            };
            const payload: FilterData = {
                products: mockProducts,
                timeframe: largeTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.timeframe?.from).toBe(9999999999999);
            expect(nextState.timeframe?.to).toBe(9999999999999);
        });

        it('should handle negative timestamp values', () => {
            const negativeTimeframe: TimeFrame = {
                type: 'RELATIVE',
                from: -1000,
                to: -500,
            };
            const payload: FilterData = {
                products: mockProducts,
                timeframe: negativeTimeframe,
            };
            
            const nextState = reducer(initialState, updateFilterData(payload));
            
            expect(nextState.timeframe?.from).toBe(-1000);
            expect(nextState.timeframe?.to).toBe(-500);
        });
    });

    describe('Slice metadata', () => {
        it('should have correct slice name', () => {
            const action = updateFilterData({});
            expect(action.type).toContain('filterData');
        });

        it('should export reducer as default', () => {
            expect(reducer).toBeDefined();
            expect(typeof reducer).toBe('function');
        });
    });
});
