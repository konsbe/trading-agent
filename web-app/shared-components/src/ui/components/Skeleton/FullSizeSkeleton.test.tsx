/// <reference types="@testing-library/jest-dom" />
import React from 'react';
import { describe, it, expect } from '@jest/globals';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import FullSizeSkeleton from './FullSizeSkeleton';
import Skeleton from './Skeleton';

describe('FullSizeSkeleton', () => {
    it('renders the wrapper with its class', () => {
        render(<FullSizeSkeleton />);

        const wrapper = screen.getByTestId('full-size-skeleton-wrapper');
        expect(wrapper).toHaveClass('full-size-skeleton-wrapper');
        expect(wrapper.tagName).toBe('DIV');
    });

    it('renders exactly one full size Skeleton', () => {
        render(<FullSizeSkeleton />);

        const skeletons = screen.getAllByTestId('ta-skeleton');
        expect(skeletons).toHaveLength(1);
        expect(skeletons[0]).toHaveStyle({ width: '100%', height: '100%' });
    });

    it('renders multiple instances independently', () => {
        const { container } = render(
            <div>
                <FullSizeSkeleton />
                <FullSizeSkeleton />
                <FullSizeSkeleton />
            </div>
        );

        expect(container.querySelectorAll('.full-size-skeleton-wrapper')).toHaveLength(3);
    });
});

describe('Skeleton', () => {
    it('is hidden from assistive technology', () => {
        render(<Skeleton />);

        expect(screen.getByTestId('ta-skeleton')).toHaveAttribute('aria-hidden', 'true');
    });

    it('applies size, radius and custom class', () => {
        render(<Skeleton width={120} height="2rem" radius="50%" className="custom" data-testid="sk" />);

        const skeleton = screen.getByTestId('sk');
        expect(skeleton).toHaveClass('ta-skeleton', 'custom');
        expect(skeleton).toHaveStyle({ width: '120px', height: '2rem', borderRadius: '50%' });
    });
});
