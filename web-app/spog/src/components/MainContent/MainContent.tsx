import { Outlet } from "react-router-dom";
import "./MainContent-styles.css";

const MainContent = () => (
    <main className="app-main" data-testid="app-main">
        <Outlet />
    </main>
);

export default MainContent;
