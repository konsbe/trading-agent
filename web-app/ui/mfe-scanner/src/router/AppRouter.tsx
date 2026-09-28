import { Route, Routes } from 'react-router-dom';
import CandidatesPage from '@/pages/CandidatesPage';
import CandidateDetailPage from '@/pages/CandidateDetailPage';

/**
 * Relative routes, mounted under `/candidates/*` (spog's route hosted, the
 * standalone bootstrap's otherwise). Links into Stock Detail use the shared
 * `stockDetailLink` / `StockDetailLink` so they carry "Back to …" state.
 */
const AppRouter = () => (
    <Routes>
        <Route index element={<CandidatesPage />} />
        <Route path=":symbol" element={<CandidateDetailPage />} />
    </Routes>
);

export default AppRouter;
