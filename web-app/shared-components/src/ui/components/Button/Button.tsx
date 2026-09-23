import React, { forwardRef } from 'react';
import { ButtonProps } from './types';
import './Button-styles.css';

const Button = forwardRef<HTMLButtonElement, ButtonProps>(({
    variant = 'secondary',
    size = 'md',
    iconOnly = false,
    fullWidth = false,
    className = '',
    type = 'button',
    children,
    ...rest
}, ref) => {
    const classes = [
        'ta-button',
        `ta-button--${variant}`,
        `ta-button--${size}`,
        iconOnly ? 'ta-button--icon-only' : '',
        fullWidth ? 'ta-button--full-width' : '',
        className,
    ].filter(Boolean).join(' ');

    return (
        <button ref={ref} type={type} className={classes} {...rest}>
            {children}
        </button>
    );
});

Button.displayName = 'Button';

export default Button;
