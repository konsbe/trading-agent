import { render, screen } from '@testing-library/react';
import ContentStatus from './ContentStatus';

describe('ContentStatus', () => {
    it('shows the status and version', () => {
        render(<ContentStatus status="draft" version="0.1.0" />);
        expect(screen.getByTestId('content-status')).toHaveTextContent('Draft · version 0.1.0');
    });
});
