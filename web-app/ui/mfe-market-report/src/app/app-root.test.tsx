import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { mfeUserDataMessageService } from 'shellSpog/userDataMessageService';
import MarketReport from './app-root';

jest.mock('@/router/AppRouter', () => {
    const { useIsHosted } = require('@/providers/HostModeContext');
    return { __esModule: true, default: () => <div>router {useIsHosted() ? 'hosted' : 'standalone'}</div> };
});

const subscribeMock = mfeUserDataMessageService.subscribe as jest.Mock;

describe('MarketReport (hosted root)', () => {
    it('subscribes to shell user data and follows the shell theme', async () => {
        subscribeMock.mockImplementation((_name: string, handler: (event: Event) => void) => {
            handler(new CustomEvent('trading-agent:user:init', { detail: { currentUser: { theme: 'dark', authenticated: true } } }));
            return jest.fn();
        });

        render(
            <MemoryRouter>
                <MarketReport />
            </MemoryRouter>
        );

        expect(screen.getByText('router hosted')).toBeInTheDocument();
        await waitFor(() => expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'dark'));
        expect(subscribeMock).toHaveBeenCalledWith('mfe-market-report', expect.any(Function));
    });

    it('renders without waiting for authentication', () => {
        subscribeMock.mockImplementation(() => jest.fn());

        render(
            <MemoryRouter>
                <MarketReport />
            </MemoryRouter>
        );

        expect(screen.getByText('router hosted')).toBeInTheDocument();
        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'light');
    });
});
