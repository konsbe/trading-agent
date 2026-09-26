import { render, screen, waitFor } from '@testing-library/react';
import { mfeUserDataMessageService } from 'shellSpog/userDataMessageService';
import Glossary from './glossary-root';
import Handbook from './handbook-root';
import MasterClass from './masterclass-root';

const subscribeMock = mfeUserDataMessageService.subscribe as jest.Mock;

describe.each([
    ['Handbook', Handbook],
    ['MasterClass', MasterClass],
    ['Glossary', Glossary],
])('%s (exposed hosted root)', (title, Root) => {
    it('renders its page hosted and follows the shell theme', async () => {
        subscribeMock.mockImplementation((_name: string, handler: (event: Event) => void) => {
            handler(new CustomEvent('trading-agent:user:init', { detail: { currentUser: { theme: 'dark', authenticated: true } } }));
            return jest.fn();
        });

        render(<Root />);

        expect(screen.getByText(`${title} content coming.`)).toBeInTheDocument();
        expect(screen.getByTestId('education-page')).toHaveClass('education-page--hosted');
        await waitFor(() => expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'dark'));
        expect(subscribeMock).toHaveBeenCalledWith('mfe-education', expect.any(Function));
    });

    it('renders without waiting for authentication', () => {
        subscribeMock.mockImplementation(() => jest.fn());

        render(<Root />);

        expect(screen.getByText(`${title} content coming.`)).toBeInTheDocument();
        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', 'light');
    });
});
