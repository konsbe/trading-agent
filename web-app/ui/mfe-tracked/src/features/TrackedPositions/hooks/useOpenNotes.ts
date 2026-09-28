import { useCallback, useState } from 'react';

/**
 * Which closed rows have their exit-reason note open. Held by the screen, so a
 * note stays open across a collapsed card or a tab switch until the user closes it.
 */
const useOpenNotes = (): [ReadonlySet<string>, (key: string) => void] => {
    const [open, setOpen] = useState<ReadonlySet<string>>(() => new Set());

    const toggle = useCallback(
        (key: string) =>
            setOpen(prev => {
                const next = new Set(prev);
                if (next.has(key)) next.delete(key);
                else next.add(key);
                return next;
            }),
        []
    );

    return [open, toggle];
};

export default useOpenNotes;
