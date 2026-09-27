import { render, screen, waitFor } from '@testing-library/react';
import { mfeUserDataMessageService } from 'shellSpog/userDataMessageService';
import ComputedSymbols from './computed-root';
import FollowedSymbols from './followed-root';

jest.mock('@/pages/FollowedSymbolsPage', () => {
    const { useIsHosted } = require('@/providers/HostModeContext');
    return { __esModule: true, default: () => <div>FollowedSymbols page {useIsHosted() ? '(hosted)' : '(standalone)'}</div> };
});
jest.mock('@/pages/ComputedSymbolsPage', () => {
    const { useIsHosted } = require('@/providers/HostModeContext');
    return { __esModule: true, default: () => <div>ComputedSymbols page {useIsHosted() ? '(hosted)' : '(standalone)'}</div> };
});

const subscribeMock = mfeUserDataMessageService.subscribe as jest.Mock;

describe.each([
    ['FollowedSymbols', FollowedSymbols],
    ['ComputedSymbols', ComputedSymbols],
])('%s (exposed hosted root)', (title, Root) => {
    it('renders its page hosted and follows the shell theme', async () => {
        subscribeMock.mockImplementation((_name: string, handler: (event: Event) => void) => {
            handler(new CustomEvent('trading-agent:user:init', { detail: { currentUser: { theme: 'dark', authenticated: true } } }));
            return jest.fn();
        });

        render(<Root />);

        expect(screen.getByText(`${title} page (hosted)`)).toBeInTheDocument();
        await waitFor(() => expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'dark'));
        expect(subscribeMock).toHaveBeenCalledWith('mfe-watchlist', expect.any(Function));
    });

    it('renders without waiting for authentication', () => {
        subscribeMock.mockImplementation(() => jest.fn());

        render(<Root />);

        expect(screen.getByText(`${title} page (hosted)`)).toBeInTheDocument();
        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'light');
    });
});
