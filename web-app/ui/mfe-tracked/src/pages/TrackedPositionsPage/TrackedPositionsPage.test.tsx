import { render, screen } from '@testing-library/react';
import TrackedPositionsPage from './TrackedPositionsPage';

describe('TrackedPositionsPage', () => {
    it('renders the title and the placeholder', () => {
        render(<TrackedPositionsPage />);

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Tracked Positions');
        expect(screen.getByTestId('tracked-placeholder')).toHaveTextContent('Tracked positions will appear here.');
    });
});
