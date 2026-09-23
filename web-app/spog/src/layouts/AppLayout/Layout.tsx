import { useCallback, useContext, useState } from "react";
import Header from "@components/Header/Header";
import MainContent from "@components/MainContent/MainContent";
import Sidebar from "@components/Sidebar/Sidebar";
import DialogModal from "@components/Modals/DialogModal/DialogModal";
import { AuthContext } from "../../providers/AuthProvider/AuthProvider";
import { BrowserHistory } from "../../providers/BrowserHistory";
import "./Layout-styles.css";

const Layout = () => {
    const [isSidebarOpen, setIsSidebarOpen] = useState(true);
    const authDataProps = useContext(AuthContext);

    const toggleSidebar = useCallback(() => setIsSidebarOpen(open => !open), []);

    return (
        <BrowserHistory>
            <div className="app-shell" data-testid="app-shell">
                <Header isSidebarOpen={isSidebarOpen} onToggleSidebar={toggleSidebar} />
                <div className="app-shell__body">
                    <Sidebar isOpen={isSidebarOpen} />
                    <MainContent />
                </div>
            </div>
            {authDataProps && <>
                <DialogModal
                    isOpen={authDataProps.refreshTokenDialogisOpen}
                    dialogTitle="Update Token"
                    dialogBody="Your token is about to expire! Would you like to refresh it now to avoid interruption?"
                    submitButton={{ content: "Yes", onSubmit: async () => await authDataProps.updateToken(), variant: "primary" }}
                    cancelButton={{ content: "No", onSubmit: () => authDataProps.logOut() }}
                />
                <DialogModal
                    isOpen={authDataProps.isExitDialogOpen}
                    dialogTitle="Sign Out"
                    dialogBody="Are you sure you want to log out?"
                    submitButton={{ content: "Log out", onSubmit: () => authDataProps.logOut(), variant: "danger" }}
                    cancelButton={{ content: "Cancel", onSubmit: () => authDataProps.setIsExitIsDialogOpen(!authDataProps.isExitDialogOpen) }}
                />
            </>}
        </BrowserHistory>
    );
};

export default Layout;
