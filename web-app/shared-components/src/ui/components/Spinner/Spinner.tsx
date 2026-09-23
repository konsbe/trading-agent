import React from 'react';
import './Spinner-styles.css';

export type SpinnerSize = 'sm' | 'md' | 'lg';

export interface SpinnerProps {
    size?: SpinnerSize;
    label?: string;
    className?: string;
    'data-testid'?: string;
}

const Spinner = ({ size = 'md', label = 'Loading', className = '', 'data-testid': testId = 'ta-spinner' }: SpinnerProps) => (
    <span
        role="status"
        aria-label={label}
        className={`ta-spinner ta-spinner--${size} ${className}`.trim()}
        data-testid={testId}
    />
);

export default Spinner;
