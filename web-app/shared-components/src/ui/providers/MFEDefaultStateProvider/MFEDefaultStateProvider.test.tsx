import React from 'react';
import { render } from '@testing-library/react';
import MFEStateProvider from './index';

// Dummy child component for testing
const DummyChild = () => <div data-testid="dummy-child">Test Child</div>;

describe('MFEStateProvider', () => {
  it('renders children correctly', () => {
    const { getByTestId } = render(
      <MFEStateProvider>
        <DummyChild />
      </MFEStateProvider>
    );
    expect(getByTestId('dummy-child')).toBeInTheDocument();
  });

  it('does not modify children or context', () => {
    const { container } = render(
      <MFEStateProvider>
        <span data-testid="plain-span">Plain</span>
      </MFEStateProvider>
    );
    expect(container.querySelector('[data-testid="plain-span"]')).not.toBeNull();
    expect(container.textContent).toBe('Plain');
  });
});
