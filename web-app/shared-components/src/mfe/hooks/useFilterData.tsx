import { useEffect, useState, useCallback } from 'react';

export interface FilterData {
    [key: string]: any;
}

const useFilterData = (mfeName: string) => {
    const [filterData, setFilterData] = useState<FilterData | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const handleFilterData = useCallback((event: Event) => {
        const data = (event as CustomEvent).detail as FilterData;
        if (data) {
            setFilterData(prev => ({ ...prev, ...data }));
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        let unsubscribe: (() => void) | null = null;

        const loadFilterData = async () => {
            try {
                const { mfeFilterDataMessageService } = await import('shellSpog/filterDataMessageService');
                unsubscribe = mfeFilterDataMessageService.subscribe(mfeName, handleFilterData);
            } catch (err) {
                setError(`Failed to connect to shell: ${(err as Error).message}`);
                setIsLoading(false);
            }
        };

        loadFilterData();

        return () => {
            if (unsubscribe) {
                unsubscribe();
            }
        };
    }, [mfeName, handleFilterData]);

    return { filterData, isLoading, error };
};

export default useFilterData;
