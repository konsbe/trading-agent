import { Route, Routes } from 'react-router-dom';
import DataSourcePage from '@/pages/DataSourcePage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `data-source/*`
 * route when hosted, so links must stay relative.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<DataSourcePage />} />
    </Routes>
);

export default AppRouter;
