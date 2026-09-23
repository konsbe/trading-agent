import { Provider } from 'react-redux';
import { getGlobalStore } from '../common/state_management/utils/globalStoreUtils';

import { AuthProvider } from '../providers/AuthProvider/AuthProvider'
import useAuth from "../hooks/useAuth"
import { FC, ReactNode } from "react";
import { MessageServiceProvider } from "../common/message_service/providers/MessageServiceProvider";
import { DndProvider } from "react-dnd";
import { HTML5Backend } from "react-dnd-html5-backend";
import SseMetadataProvider from "../providers/SseMetadataProvider/SseMetadataProvider";
import { ThemeProvider } from "../providers/ThemeProvider";

// Initialize global redux store
const store = getGlobalStore();

const AppWrapper: FC<{ children : ReactNode }> = ({ children }) => {
  const { keycloak, setAuthData, initialized } = useAuth()
  return (
    <DndProvider backend={HTML5Backend}>
      <ThemeProvider>
          <Provider store={store}>
            <MessageServiceProvider>
              <AuthProvider authData={keycloak} setAuthData={setAuthData} loading={!initialized}>
                <SseMetadataProvider>
                  { children }
                </SseMetadataProvider>
              </AuthProvider>
            </MessageServiceProvider>
          </Provider>
      </ThemeProvider>
    </DndProvider>
  )
}

export default AppWrapper;