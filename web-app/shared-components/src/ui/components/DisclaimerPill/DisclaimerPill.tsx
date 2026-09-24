import React from 'react';
import { DisclaimerPillProps } from './types';
import './DisclaimerPill-styles.css';

export const DISCLAIMER_TEXT = 'Screener — not a forecast';

/** Persistent "Screener — not a forecast" pill; must be visible on every screen. */
const DisclaimerPill = ({ className = '', 'data-testid': testId = 'disclaimer-pill' }: DisclaimerPillProps) => (
    <span className={`ta-disclaimer-pill ${className}`.trim()} data-testid={testId}>
        {DISCLAIMER_TEXT}
    </span>
);

export default DisclaimerPill;
