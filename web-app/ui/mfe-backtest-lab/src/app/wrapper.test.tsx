import { render, screen } from '@testing-library/react';
import { AppWrapper } from './wrapper';

describe('AppWrapper', () => {
    it.each([
        [{ theme: 'dark' }, 'dark'],
        [{ theme: 'light' }, 'light'],
        [{ theme: 'purple' }, 'light'],
        [null, 'light'],
    ])('applies theme for userData %j', (userData, expected) => {
        render(<AppWrapper userData={userData}><span>child</span></AppWrapper>);

        expect(screen.getByText('child')).toBeInTheDocument();
        expect(screen.getByTestId('ta-theme-root')).toHaveAttribute('data-theme', expected);
    });
});
