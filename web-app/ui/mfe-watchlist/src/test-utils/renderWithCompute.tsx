import { ReactElement } from 'react';
import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ComputeStatusProvider } from '@/providers/ComputeStatusContext';
import { HostModeProvider } from '@/providers/HostModeContext';

/**
 * Renders inside a router, host mode and a ComputeStatusProvider. The test
 * file must `jest.mock('@/api/tracking/trackingApi', …)` and resolve
 * `fetchComputedSymbols` before rendering.
 */
export const renderWithCompute = (ui: ReactElement, { hosted = false } = {}) =>
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter>
                <ComputeStatusProvider>{ui}</ComputeStatusProvider>
            </MemoryRouter>
        </HostModeProvider>
    );
