import { useEffect, useState } from 'react';

const useAuthMFE = (mfeName: string) => {
    const [userData, setUserData] = useState<any>(null);
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let unsubscribe: (() => void) | null = null;

        const loadUserData = async () => {
            try {
                const { mfeUserDataMessageService } = await import('shellSpog/userDataMessageService');

                const handleUserData = (event: Event) => {
                    const customEvent = event as CustomEvent;
                    const data = customEvent.detail;

                    let currentUserData = data;
                    if (data?.currentUser) {
                        currentUserData = data.currentUser;
                    }

                    if (currentUserData) {
                        setUserData(currentUserData);
                        setIsAuthenticated(!!currentUserData?.authenticated);
                        setIsLoading(false);
                    }
                };

                unsubscribe = mfeUserDataMessageService.subscribe(mfeName, handleUserData);
            } catch (err) {
                setError(`Failed to connect to shell: ${(err as Error).message}`);
                setIsLoading(false);
            }
        };

        loadUserData();

        return () => {
            if (unsubscribe) {
                unsubscribe();
            }
        };
    }, [mfeName]);

    return { userData, isAuthenticated, isLoading, error };
};

export default useAuthMFE;
