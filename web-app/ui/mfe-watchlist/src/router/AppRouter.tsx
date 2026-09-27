import { Route, Routes } from 'react-router-dom';
import ComputedSymbolsPage from '@/pages/ComputedSymbolsPage';
import FollowedSymbolsPage from '@/pages/FollowedSymbolsPage';
import WatchlistPage from '@/pages/WatchlistPage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `watchlist/*`
 * route when hosted, so links must stay relative. Hosted, spog mounts the
 * Followed / Computed Symbols roots directly under their own routes; the two
 * paths here give standalone dev (http://localhost:3002) the same screens.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<WatchlistPage />} />
        <Route path="followed-symbols/*" element={<FollowedSymbolsPage />} />
        <Route path="computed-symbols/*" element={<ComputedSymbolsPage />} />
    </Routes>
);

export default AppRouter;
