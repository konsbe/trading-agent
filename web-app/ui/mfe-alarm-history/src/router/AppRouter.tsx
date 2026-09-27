import { Route, Routes } from 'react-router-dom';
import AlarmHistoryPage from '@/pages/AlarmHistoryPage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `alarm-history/*`
 * route when hosted, so links must stay relative.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<AlarmHistoryPage />} />
    </Routes>
);

export default AppRouter;
