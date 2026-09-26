import { render, screen } from '@testing-library/react';
import { masterClassFixture } from '@/test-utils/fixtures';
import MasterClassModule from './MasterClassModule';

describe('MasterClassModule', () => {
    it('titles a numbered module "Module N" and renders its entries', () => {
        render(<MasterClassModule module={masterClassFixture().modules[0]} />);

        const region = screen.getByRole('region', { name: 'Module 3 Technical indicators, standards-based' });
        expect(region).toHaveAttribute('id', 'module-3');
        expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(/^Technical indicators, standards-based$/);
        expect(screen.getAllByTestId('masterclass-entry').map(e => e.id)).toEqual(['rsi', 'macd']);
    });

    it('omits the module label when unnumbered', () => {
        render(<MasterClassModule module={masterClassFixture().modules[1]} />);
        expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(/^Fundamental analysis$/);
    });
});
