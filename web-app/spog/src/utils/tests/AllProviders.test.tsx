import React from 'react';
import { screen } from '@testing-library/react';
import { customRenderWithAllProviders, getMockKeycloakInstance } from './AllProviders';

describe('utils/tests/AllProviders', () => {
  it('creates a mocked Keycloak instance with the provided name', () => {
    const kc = getMockKeycloakInstance('John Smith');
    expect(kc.authenticated).toBe(true);
    expect(kc.token).toBe('fake-token');
    expect(kc.tokenParsed.name).toBe('John Smith');
    expect(typeof kc.isTokenExpired).toBe('function');
    expect(typeof kc.login).toBe('function');
    expect(typeof kc.logout).toBe('function');
  });

  it('customRenderWithAllProviders renders children', () => {
    customRenderWithAllProviders(<div>hello</div>);
    expect(screen.getByText('hello')).toBeInTheDocument();
  });

  it('customRenderWithAllProviders accepts userName option (smoke)', () => {
    customRenderWithAllProviders(<div>hello</div>, { userName: 'Jane Doe' });
    expect(screen.getByText('hello')).toBeInTheDocument();
  });
});
