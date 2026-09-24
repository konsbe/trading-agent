import { Route, Routes } from 'react-router-dom';
import WatchlistPage from '@/pages/WatchlistPage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `watchlist/*`
 * route when hosted, so links must stay relative.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<WatchlistPage />} />
    </Routes>
);

export default AppRouter;
