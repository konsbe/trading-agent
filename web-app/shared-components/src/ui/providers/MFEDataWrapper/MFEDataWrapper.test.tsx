import React from 'react';
import { render, screen, cleanup } from '@testing-library/react';
import '@testing-library/jest-dom';
import MFEDataWrapper from './index';

describe('MFEDataWrapper', () => {
  afterEach(() => {
    cleanup();
  });

  it('shows loading state when dataLoading is true', () => {
    render(
      <MFEDataWrapper dataLoading={true} data={[{ id: 1 }]} dataError={null}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('Loading data...')).toBeInTheDocument();
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  it('shows error state when dataError is set', () => {
    render(
      <MFEDataWrapper
        dataLoading={false}
        dataError={new Error('fail')}
        data={[{ id: 1 }]}
      >
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(
      screen.getByText(/There was a problem trying to fetch your data/)
    ).toBeInTheDocument();
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  it('does not append raw error message in development', () => {
    const originalEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    render(
      <MFEDataWrapper dataLoading={false} dataError={new Error('Failed to fetch')}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(
      screen.getByText('There was a problem trying to fetch your data.')
    ).toBeInTheDocument();
    expect(screen.queryByText(/Failed to fetch/)).not.toBeInTheDocument();

    process.env.NODE_ENV = originalEnv;
  });

  it('shows no data state when data is empty', () => {
    render(
      <MFEDataWrapper dataLoading={false} dataError={null} data={[]}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('No data available')).toBeInTheDocument();
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  it('shows custom noDataMessage', () => {
    render(
      <MFEDataWrapper
        dataLoading={false}
        dataError={null}
        data={[]}
        noDataMessage="No roles found"
      >
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('No roles found')).toBeInTheDocument();
  });

  it('supports nodDataMessage typo alias from topology-groups', () => {
    render(
      <MFEDataWrapper dataLoading={false} dataError={null} data={[]} nodDataMessage="Topology not configured">
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('Topology not configured')).toBeInTheDocument();
  });

  it('renders children when noDataDeactivated is true even if data is empty', () => {
    render(
      <MFEDataWrapper
        dataLoading={false}
        dataError={null}
        data={[]}
        noDataDeactivated
      >
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByTestId('child')).toBeInTheDocument();
    expect(screen.queryByText('No data available')).not.toBeInTheDocument();
  });

  it('renders children when data is provided and not loading or error', () => {
    render(
      <MFEDataWrapper dataLoading={false} dataError={null} data={[{ id: 1, name: 'admin' }]}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });

  it('shows no data when first array row has empty name (topology-style)', () => {
    render(
      <MFEDataWrapper
        dataLoading={false}
        dataError={null}
        data={[{ name: '', id: 'n1' }]}
      >
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('No data available')).toBeInTheDocument();
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  it('maps deprecated isLoading to dataLoading', () => {
    render(
      <MFEDataWrapper isLoading={true} data={null}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('Loading data...')).toBeInTheDocument();
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  it('maps deprecated isError and errorMessage to dataError', () => {
    render(
      <MFEDataWrapper isError={true} errorMessage={new Error('legacy')}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(
      screen.getByText(/There was a problem trying to fetch your data/)
    ).toBeInTheDocument();
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  it('shows children when dataExist is true without array data', () => {
    render(
      <MFEDataWrapper data={null} dataExist={true} isLoading={false} isError={false}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });

  it('renders children when array rows omit name (non-topology data)', () => {
    render(
      <MFEDataWrapper dataLoading={false} dataError={null} data={[{ id: 'role-1' }]}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });

  it('shows no data when data is null', () => {
    render(
      <MFEDataWrapper dataLoading={false} dataError={null} data={null}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('No data available')).toBeInTheDocument();
  });

  it('shows no data when data is boolean false', () => {
    render(
      <MFEDataWrapper dataLoading={false} dataError={null} data={false}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('No data available')).toBeInTheDocument();
  });

  it('renders children when data is boolean true', () => {
    render(
      <MFEDataWrapper dataLoading={false} dataError={null} data={true}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });

  it('maps deprecated isError with string errorMessage', () => {
    render(
      <MFEDataWrapper isError={true} errorMessage="Network timeout">
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(
      screen.getByText(/There was a problem trying to fetch your data/)
    ).toBeInTheDocument();
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  it('maps deprecated isError without errorMessage to default error', () => {
    render(
      <MFEDataWrapper isError={true}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(
      screen.getByText(/There was a problem trying to fetch your data/)
    ).toBeInTheDocument();
  });

  it('applies emptyStateStackStyle to the illustration stack', () => {
    const { container } = render(
      <MFEDataWrapper data={[]} emptyStateStackStyle={{ gap: '10px' }}>
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );

    const stack = container.querySelector('div > div > div');
    expect(stack).toHaveStyle({ gap: '10px' });
  });

  it('omits illustration when showEmptyIllustration is false', () => {
    render(
      <MFEDataWrapper
        dataLoading={false}
        dataError={null}
        data={[]}
        showEmptyIllustration={false}
      >
        <span data-testid="child">content</span>
      </MFEDataWrapper>
    );
    expect(screen.getByText('No data available')).toBeInTheDocument();
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });
});
