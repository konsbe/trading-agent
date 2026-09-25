import { Route, Routes } from 'react-router-dom';
import MarketReportPage from '@/pages/MarketReportPage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `market-report/*`
 * route when hosted, so links must stay relative.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<MarketReportPage />} />
    </Routes>
);

export default AppRouter;
