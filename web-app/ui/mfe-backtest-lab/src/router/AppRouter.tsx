import { Route, Routes } from 'react-router-dom';
import BacktestLabPage from '@/pages/BacktestLabPage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `backtest-lab/*`
 * route when hosted, so links must stay relative.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<BacktestLabPage />} />
    </Routes>
);

export default AppRouter;
