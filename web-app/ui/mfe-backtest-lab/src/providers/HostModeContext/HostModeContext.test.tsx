import { render, screen } from '@testing-library/react';
import { HostModeProvider, useIsHosted } from './HostModeContext';

const Probe = () => <span data-testid="probe">{useIsHosted() ? 'hosted' : 'standalone'}</span>;

describe('HostModeContext', () => {
    it('defaults to standalone without a provider', () => {
        render(<Probe />);
        expect(screen.getByTestId('probe')).toHaveTextContent('standalone');
    });

    it.each([
        [true, 'hosted'],
        [false, 'standalone'],
    ])('provides hosted=%s', (hosted, expected) => {
        render(
            <HostModeProvider hosted={hosted}>
                <Probe />
            </HostModeProvider>
        );
        expect(screen.getByTestId('probe')).toHaveTextContent(expected);
    });
});
