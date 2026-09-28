import { act, screen } from '@testing-library/react';
import { mount } from './bootstrap';

jest.mock('@/router/AppRouter', () => {
    const { useIsHosted } = require('@/providers/HostModeContext');
    return { __esModule: true, default: () => <div>{useIsHosted() ? 'hosted-router' : 'standalone-router'}</div> };
});

describe('bootstrap (standalone)', () => {
    it('mounts the standalone app with its own router and theme', () => {
        const container = document.createElement('div');
        document.body.appendChild(container);

        let root: ReturnType<typeof mount> | undefined;
        act(() => {
            root = mount(container);
        });

        expect(screen.getByText('standalone-router')).toBeInTheDocument();
        expect(screen.getByTestId('ta-theme-root')).toBeInTheDocument();
        act(() => root?.unmount());
    });

    it('throws when the root container is missing', () => {
        expect(() => mount(null)).toThrow('Root container missing');
    });
});
