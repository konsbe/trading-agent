import { createContext, ReactNode, useCallback, useContext } from 'react';

const TypeLabelsContext = createContext<Readonly<Record<string, string>>>({});

/** The API's `type_labels` for everything below it (filter chips, tables, group rows). */
export const TypeLabelsProvider = ({ labels, children }: { labels: Readonly<Record<string, string>>; children: ReactNode }) => (
    <TypeLabelsContext.Provider value={labels}>{children}</TypeLabelsContext.Provider>
);

/** `alert_type` → its label from the API; a type the API has no label for shows its id as-is. */
export const useTypeLabel = (): ((alertType: string) => string) => {
    const labels = useContext(TypeLabelsContext);
    return useCallback((alertType: string) => labels[alertType] ?? alertType, [labels]);
};
