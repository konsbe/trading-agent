import { render, screen } from '@testing-library/react';
import { BrowserRouter, MemoryRouter } from 'react-router-dom';
import UserAccessControl from './UserAccessControl';
import { useRoleAuth } from '../../hooks/useRolesAuth';
import { ROLES } from '../../constants/roles';

// Mock the useRoleAuth hook
jest.mock('../../hooks/useRolesAuth');
const mockUseRoleAuth = useRoleAuth as jest.MockedFunction<typeof useRoleAuth>;

// Mock Navigate component from react-router-dom
const mockNavigate = jest.fn();
jest.mock('react-router-dom', () => ({
  ...jest.requireActual('react-router-dom'),
  Navigate: ({ to }: { to: string }) => {
    mockNavigate(to);
    return <div data-testid="navigate-mock">Redirecting to {to}</div>;
  },
}));

describe('UserAccessControl', () => {
  const TestChildren = () => <div data-testid="test-children">Protected Content</div>;

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders the loading message when isLoading is true', () => {
    mockUseRoleAuth.mockReturnValue({
      canAccess: false,
      isLoading: true,
    });

    render(
      <BrowserRouter>
        <UserAccessControl roles={[ROLES.ADMIN]}>
          <TestChildren />
        </UserAccessControl>
      </BrowserRouter>
    );

    expect(screen.getByText('Loading User Authorization...')).toBeInTheDocument();
    expect(screen.queryByTestId('test-children')).not.toBeInTheDocument();
    expect(screen.queryByTestId('navigate-mock')).not.toBeInTheDocument();

    expect(screen.getByTestId('user-access-control-loading')).toHaveClass('user-access-control__loading');
  });

  it('renders children when user has access', () => {
    mockUseRoleAuth.mockReturnValue({
      canAccess: true,
      isLoading: false,
    });

    render(
      <BrowserRouter>
        <UserAccessControl roles={[ROLES.TRADER]}>
          <TestChildren />
        </UserAccessControl>
      </BrowserRouter>
    );

    expect(screen.getByTestId('test-children')).toBeInTheDocument();
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
    expect(screen.queryByText('Loading User Authorization...')).not.toBeInTheDocument();
    expect(screen.queryByTestId('navigate-mock')).not.toBeInTheDocument();
  });

  it('redirects to unauthorized page when user does not have access', () => {
    mockUseRoleAuth.mockReturnValue({
      canAccess: false,
      isLoading: false,
    });

    render(
      <MemoryRouter>
        <UserAccessControl roles={[ROLES.ADMIN]}>
          <TestChildren />
        </UserAccessControl>
      </MemoryRouter>
    );

    expect(screen.getByTestId('navigate-mock')).toBeInTheDocument();
    expect(screen.getByText('Redirecting to /unauthorized')).toBeInTheDocument();
    expect(mockNavigate).toHaveBeenCalledWith('/unauthorized');
    expect(screen.queryByTestId('test-children')).not.toBeInTheDocument();
  });

  it('calls useRoleAuth with correct roles parameter', () => {
    const testRoles = [ROLES.ADMIN, ROLES.VIEWER];
    mockUseRoleAuth.mockReturnValue({
      canAccess: true,
      isLoading: false,
    });

    render(
      <BrowserRouter>
        <UserAccessControl roles={testRoles}>
          <TestChildren />
        </UserAccessControl>
      </BrowserRouter>
    );

    expect(mockUseRoleAuth).toHaveBeenCalledWith({
      requiredRoles: testRoles,
    });
  });

  it('handles different children types correctly', () => {
    mockUseRoleAuth.mockReturnValue({
      canAccess: true,
      isLoading: false,
    });

    // Test with multiple children
    const { rerender } = render(
      <BrowserRouter>
        <UserAccessControl roles={[ROLES.VIEWER]}>
          <div data-testid="child-1">Child 1</div>
          <span data-testid="child-2">Child 2</span>
        </UserAccessControl>
      </BrowserRouter>
    );

    expect(screen.getByTestId('child-1')).toBeInTheDocument();
    expect(screen.getByTestId('child-2')).toBeInTheDocument();

    // Test with string children
    rerender(
      <BrowserRouter>
        <UserAccessControl roles={[ROLES.VIEWER]}>
          Plain text content
        </UserAccessControl>
      </BrowserRouter>
    );

    expect(screen.getByText('Plain text content')).toBeInTheDocument();

    // Test with null children
    rerender(
      <BrowserRouter>
        <UserAccessControl roles={[ROLES.VIEWER]}>
          {null}
        </UserAccessControl>
      </BrowserRouter>
    );

    expect(screen.queryByTestId('child-1')).not.toBeInTheDocument();
  });

  it('handles empty roles array', () => {
    mockUseRoleAuth.mockReturnValue({
      canAccess: true,
      isLoading: false,
    });

    render(
      <BrowserRouter>
        <UserAccessControl roles={[]}>
          <TestChildren />
        </UserAccessControl>
      </BrowserRouter>
    );

    expect(mockUseRoleAuth).toHaveBeenCalledWith({
      requiredRoles: [],
    });
    expect(screen.getByTestId('test-children')).toBeInTheDocument();
  });
});