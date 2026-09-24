import { Route, Routes } from 'react-router-dom';
import CandidatesPage from '@/pages/CandidatesPage';
import CandidateDetailPage from '@/pages/CandidateDetailPage';

/**
 * Relative routes: mounted at `/` standalone and under spog's `candidates/*`
 * route when hosted, so links must stay relative (`to={symbol}`, `to=".."`).
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<CandidatesPage />} />
        <Route path=":symbol" element={<CandidateDetailPage />} />
    </Routes>
);

export default AppRouter;
