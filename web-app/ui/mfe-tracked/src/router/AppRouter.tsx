import { Route, Routes } from 'react-router-dom';
import TrackedPositionsPage from '@/pages/TrackedPositionsPage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `tracked-positions/*`
 * route when hosted, so links must stay relative.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<TrackedPositionsPage />} />
    </Routes>
);

export default AppRouter;
