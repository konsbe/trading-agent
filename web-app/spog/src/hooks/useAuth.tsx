import { useEffect, useState } from "react";
import Keycloak, { KeycloakInstance, KeycloakInitOptions, KeycloakError } from 'keycloak-js';

interface UseAuthReturn {
    keycloak: KeycloakInstance | null;
    setAuthData: React.Dispatch<React.SetStateAction<KeycloakInstance | null>>;
    initialized: boolean;
}

const useAuth = (): UseAuthReturn => {
    const [initialized, setInitialized] = useState<boolean>(false);
    const [kc, setKc] = useState<KeycloakInstance | null>(null);

    useEffect(() => {
        if (!initialized && !process.env.IS_TEST_ENV) {

            const shellConfig = window.__APP_CONFIG__?.shell_spog?.config;
            const keycloakUrl = shellConfig?.keycloakUrl;
            if (!keycloakUrl) {
                return;
            }

            if (!window.crypto || !window.crypto.subtle) {
                return;
            }

            const keycloak: KeycloakInstance = new (Keycloak as any)({
                url: keycloakUrl,
                realm: shellConfig?.keycloakRealm,
                clientId: shellConfig?.keycloakClientId,
            });

            const initOptions: KeycloakInitOptions = {
                onLoad: "login-required",
                pkceMethod: "S256",
                checkLoginIframe: false,
                enableLogging: false,
            };

            keycloak
                .init(initOptions)
                .then((authenticated: any) => {
                    if (!authenticated) {
                        keycloak.login();
                        return;
                    } else {
                        keycloak.onTokenExpired = () => {
                            keycloak.updateToken(5);
                        };
                        setKc(keycloak);
                        setInitialized(true);
                    }
                })
                .catch((err: KeycloakError) => {
                    // Logout user
                    keycloak.logout();
                });
        }
    }, [initialized]);

    return { keycloak: kc, setAuthData: setKc, initialized };
};

export default useAuth;
