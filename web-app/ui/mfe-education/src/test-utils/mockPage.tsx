import { useIsHosted } from '@/providers/HostModeContext';

/** `jest.mock` module for a page: renders "<name> page", plus " (hosted)" inside HostModeProvider hosted. */
export const mockPageModule = (name: string) => ({
    __esModule: true,
    default: () => {
        const hosted = useIsHosted();
        return <p>{`${name} page${hosted ? ' (hosted)' : ''}`}</p>;
    },
});
