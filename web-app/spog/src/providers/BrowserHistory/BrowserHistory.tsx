import React, { createContext, useContext, useEffect, useRef, useState, useCallback } from "react";
import { useLocation } from "react-router-dom";

interface BrowserHistoryContextType {
  historyStack: string[];
  pushPath: (path: string) => void;
  popPath: () => void;
}

const BrowserHistoryContext = createContext<BrowserHistoryContextType | undefined>(undefined);

// Added BrowserHistory context provider to track navigation history across the app
// BrowserHistory needs to be inside of create browser this is the reason we are not putting it inside the wrappers
export const BrowserHistoryProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const location = useLocation();
    const [historyStack, setHistoryStack] = useState<string[]>([location.pathname + location.search]);

    // Only push if the path is different from the last
    const pushPath = useCallback((path: string) => {
        setHistoryStack(prev => {
            if (prev[prev.length - 1] !== path) {
                return [...prev, path];
            }
            return prev;
        });
    }, []);

    const popPath = useCallback(() => {
        setHistoryStack(prev => {
            if (prev.length > 1) {
                return prev.slice(0, -1);
            }
            return prev;
        });
    }, []);

    useEffect(() => {
        pushPath(location.pathname + location.search);
    }, [location.pathname, location.search, pushPath]);

    return (
        <BrowserHistoryContext.Provider value={{ historyStack, pushPath, popPath }}>
            {children}
        </BrowserHistoryContext.Provider>
    );
};

export const useBrowserHistory = () => {
    const browserHistoryContext = useContext(BrowserHistoryContext);
    if (!browserHistoryContext) throw new Error("useBrowserHistory must be used within a BrowserHistoryProvider");
    return browserHistoryContext;
};

export default BrowserHistoryProvider;