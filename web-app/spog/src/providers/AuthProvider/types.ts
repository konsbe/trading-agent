import { ReactNode, Dispatch, SetStateAction } from "react";
import { KeycloakInstance } from 'keycloak-js';

export interface TokenParsed {
  exp: number;
  iat?: number;
  [key: string]: any;
}

export interface AuthData {
  token: string;
  tokenParsed: TokenParsed | null;
  refreshTokenParsed?: TokenParsed | null;
  authenticated: boolean;
  updateToken: (minValidity: number) => Promise<boolean>;
  logout: (options?: any) => void;
  hasRealmRole: (role: string) => boolean;
  hasResourceRole: (role: string) => boolean;
  isTokenExpired: (minValidity: number) => boolean;
}

export interface AuthContextProps {
  setRefreshTokenDialogisOpen: Dispatch<SetStateAction<boolean>>,
  setIsExitIsDialogOpen: Dispatch<SetStateAction<boolean>>,
  isExitDialogOpen: boolean,
  refreshTokenDialogisOpen: boolean,
  isKeycloakAuthenticated: () => void;
  showAccessToken: () => void;
  showParsedToken: () => void;
  checkTokenIfExpired: () => void;
  updateToken: () => Promise<void>;
  logOut: () => void;
  hasRealmRole: (role: string) => void;
  hasResourceRole: (role: string) => void;
  openExitDialogModal: () => void;
  tokenParsed: TokenParsed | null;
  userName: string;
  userRoles: string[];
}


export interface AuthProviderProps {
  children: ReactNode;
  authData: AuthData | KeycloakInstance | null;
  setAuthData: Dispatch<SetStateAction<KeycloakInstance | null>>;
  loading: boolean;
}

export type PartialAuthContextProps = Partial<AuthContextProps> &
  Pick<
    AuthContextProps,
    'updateToken' | 'setRefreshTokenDialogisOpen'| 'logOut' | 'tokenParsed' | 'userName' | 'userRoles'
    | 'setIsExitIsDialogOpen' | 'refreshTokenDialogisOpen' | 'isExitDialogOpen' | 'openExitDialogModal'
  >;