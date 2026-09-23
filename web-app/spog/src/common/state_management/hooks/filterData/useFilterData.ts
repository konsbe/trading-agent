import { useSelector } from 'react-redux';
import { RootState } from '../../store/dataStore';
import { FilterData, Product, TimeFrame } from '../../slices/filterData/filterDataSlice';

export const useFilterData = (): FilterData | null => {
    return useSelector((state: RootState) => state.filterData) || null;
};

export const useFilterDataProducts = (): Array<Product> => {
    const filterData = useSelector((state: RootState) => state.filterData);
    return filterData?.products || [];
};

export const useFilterDataTimeframe = (): TimeFrame | null => {
    const filterData = useSelector((state: RootState) => state.filterData);
    return filterData?.timeframe || null;
};
