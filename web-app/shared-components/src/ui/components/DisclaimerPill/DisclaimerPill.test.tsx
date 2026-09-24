import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import DisclaimerPill, { DISCLAIMER_TEXT } from './DisclaimerPill';
import * as indexExports from '../../../index';
import * as mfeExports from '../../../mfe';
import * as shellExports from '../../../shell';

describe('DisclaimerPill', () => {
    it('renders the disclaimer copy with the pill class and default test id', () => {
        render(<DisclaimerPill />);

        const pill = screen.getByTestId('disclaimer-pill');
        expect(pill).toHaveTextContent('Screener — not a forecast');
        expect(pill).toHaveTextContent(DISCLAIMER_TEXT);
        expect(pill).toHaveClass('ta-disclaimer-pill');
    });

    it('applies a custom class and test id', () => {
        render(<DisclaimerPill className="extra" data-testid="header-pill" />);

        expect(screen.getByTestId('header-pill')).toHaveClass('ta-disclaimer-pill', 'extra');
    });

    it.each([
        ['index', indexExports],
        ['mfe', mfeExports],
        ['shell', shellExports],
    ])('is exported from the %s barrel', (_name, barrel) => {
        expect(barrel.DisclaimerPill).toBe(DisclaimerPill);
        expect(barrel.DISCLAIMER_TEXT).toBe(DISCLAIMER_TEXT);
    });
});
